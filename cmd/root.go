package cmd

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/izetmolla/containerws/config"
	"github.com/izetmolla/containerws/modules"
	"github.com/izetmolla/containerws/modules/mcp"
	settingsupdate "github.com/izetmolla/containerws/modules/settings/update"
	"github.com/izetmolla/containerws/packages/usersync"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"github.com/valyala/fasthttp"
)

var (
	flagNamesMigrations = map[string]string{}
	warnedFlags         = map[string]bool{}
)

// TODO(remove): remove after July 2026.
func migrateFlagNames(_ *pflag.FlagSet, name string) pflag.NormalizedName {
	if newName, ok := flagNamesMigrations[name]; ok {
		if !warnedFlags[name] {
			warnedFlags[name] = true
			log.Printf("DEPRECATION NOTICE: Flag --%s has been deprecated, use --%s instead\n", name, newName)
		}
		name = newName
	}
	return pflag.NormalizedName(name)
}

func init() {
	rootCmd.SilenceUsage = true
	rootCmd.SetGlobalNormalizationFunc(migrateFlagNames)

	cobra.MousetrapHelpText = ""

	rootCmd.SetVersionTemplate("Container Workspace version {{printf \"%s\" .Version}}\n")

	persistent := rootCmd.PersistentFlags()
	persistent.StringP("config", "c", "", "config file path")
	persistent.StringP("database", "d", "", "database path (optional; defaults from ENV)")

	flags := rootCmd.Flags()
	flags.Bool("start", false, "start the API/UI server (foreground)")
	flags.Bool("sync-users", false, "sync Linux ↔ panel users on start (default: on when ENV=production; override with SYNC_USERS)")
	flags.Bool("noauth", false, "use the noauth auther when using quick setup")
	flags.String("username", "admin", "username for the first user when using quick setup")
	flags.String("password", "", "hashed password for the first user when using quick setup")
	addServerFlags(flags)
}

// addServerFlags adds server related flags to the given FlagSet. These flags are available
// in both the root command, config set and config init commands.
func addServerFlags(flags *pflag.FlagSet) {
	_ = flags
}

var rootCmd = &cobra.Command{
	Use:     "cws",
	Aliases: []string{"containerws"},
	Short:   "Container Workspace server and CLI",
	Long: `Container Workspace CLI — manage users/config, or start the API/UI server.

Run with no arguments to show this help. Start the server with:

  cws --start

(Also available as containerws.)

Listens on :9000 (HTTP by default). Set ENABLE_HTTPS=true to serve HTTPS on the
same port; a self-signed cert is generated and stored in os_environments
(SERVER_SSL_CERTIFICATE / SERVER_SSL_KEY) under /config/containerws/ssl/ (or
./tmp/ssl in development).

Optional MCP standalone listener: set MCP_PORT (e.g. 9100) to expose MCP at
http://host:MCP_PORT/ in addition to /api/mcp on the main port. Auth uses
mcp_keys and/or MCP_TOKEN.

On --start, Linux ↔ panel users can be synced (see --sync-users / SYNC_USERS):
Linux login accounts missing from the DB are inserted; panel users with a
username missing on Linux get a full account (home + bash). Default: enabled
when ENV=production.

Subcommands (users, software, vnc, version, …) work without --start.

Flags are also available as environment variables prefixed by "CW_"
(UPPER_SNAKE_CASE), except "--config".

Configuration precedence: flags → environment → config file → database → defaults.`,
	Args: cobra.NoArgs,
	RunE: withViperAndStore(func(cmd *cobra.Command, _ []string, v *viper.Viper, appClients *config.AppClients) error {
		start, err := cmd.Flags().GetBool("start")
		if err != nil {
			return err
		}
		if start {
			return runServer(appClients, shouldSyncUsers(cmd))
		}
		return cmd.Help()

	}),
}

func shouldSyncUsers(cmd *cobra.Command) bool {
	if cmd != nil && cmd.Flags().Changed("sync-users") {
		v, _ := cmd.Flags().GetBool("sync-users")
		return v
	}
	if v := strings.TrimSpace(os.Getenv("SYNC_USERS")); v != "" {
		switch strings.ToLower(v) {
		case "1", "true", "yes", "on":
			return true
		case "0", "false", "no", "off":
			return false
		}
	}
	return strings.EqualFold(strings.TrimSpace(os.Getenv("ENV")), "production")
}

func runServer(appClients *config.AppClients, syncUsers bool) error {
	serverConfig := appClients.ServerConfig()

	if syncUsers {
		usersync.StartAsync(appClients.DB())
	} else {
		log.Printf("usersync: skipped (pass --sync-users or set SYNC_USERS=true)")
	}

	settingsupdate.StartBackgroundChecks()

	app := fiber.New(fiber.Config{
		// Cloudflare Tunnel / reverse proxies speak HTTP to the origin while
		// the browser uses HTTPS. Trust loopback + private peers so Scheme(),
		// Hostname(), and IP() honor X-Forwarded-* (required for wss://).
		TrustProxy: true,
		TrustProxyConfig: fiber.TrustProxyConfig{
			Loopback:  true,
			Private:   true,
			LinkLocal: true,
		},
		ProxyHeader: fiber.HeaderXForwardedFor,
	})
	modules.SetupRoutes(app, appClients)
	handler := app.Handler()

	// Optional dedicated MCP listener — DB settings first, else MCP_PORT env.
	mcpReady := make(chan *fiber.App, 1)
	go func() {
		app, err := mcp.StartStandaloneFromSettingsOrEnv(appClients)
		if err != nil {
			log.Printf("MCP standalone: %v", err)
		}
		mcpReady <- app
	}()
	select {
	case <-mcpReady:
	case <-time.After(3 * time.Second):
		log.Printf("MCP standalone: still starting (may be disabled or slow)")
	}

	addr := net.JoinHostPort(serverConfig.ADDRESS, serverConfig.PORT)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("listen %s: %w", addr, err)
	}

	scheme := "http"
	if serverConfig.ENABLE_HTTPS {
		cert, err := config.ResolveTLSCertificate(context.Background(), serverConfig, appClients.Environments())
		if err != nil {
			_ = ln.Close()
			return fmt.Errorf("tls certificate: %w", err)
		}
		ln = tls.NewListener(ln, &tls.Config{
			Certificates: []tls.Certificate{cert},
			MinVersion:   tls.VersionTLS12,
		})
		scheme = "https"
	}

	listenErr := make(chan error, 1)
	go func() {
		listenErr <- fasthttp.Serve(ln, handler)
	}()

	if err := waitForListen(addr, 5*time.Second); err != nil {
		_ = ln.Close()
		return fmt.Errorf("server failed on %s: %w", addr, err)
	}

	if viper.GetBool("start") {
		log.Printf("%s listening on %s://%s", strings.ToUpper(scheme), scheme, addr)
	}
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-listenErr:
		_ = ln.Close()
		if err != nil {
			return fmt.Errorf("server error: %w", err)
		}
		return nil
	case <-quit:
	}

	fmt.Println("\nShutting down server...")

	_ = ln.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := mcp.StopStandalone(); err != nil {
		log.Printf("MCP standalone shutdown failed: %v", err)
	}

	if err := app.ShutdownWithContext(ctx); err != nil {
		log.Printf("Server shutdown failed: %v", err)
	}

	if appClients.Environments() != nil {
		appClients.Environments().Close()
	}

	db, err := appClients.DB().DB()
	if err == nil {
		db.Close()
	}

	fmt.Println("Server gracefully stopped")
	return nil
}

func waitForListen(addr string, timeout time.Duration) error {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return err
	}
	if host == "" || host == "0.0.0.0" {
		host = "127.0.0.1"
	}

	dialAddr := net.JoinHostPort(host, port)
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", dialAddr, 200*time.Millisecond)
		if err == nil {
			conn.Close()
			return nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	return fmt.Errorf("timeout waiting for listener")
}
