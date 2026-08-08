package mcp

import (
	"fmt"
	"log"
	"net"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/izetmolla/containerws/config"
	"github.com/spf13/viper"
)

func tokenMiddleware(appClients *config.AppClients, token string) fiber.Handler {
	cc := NewController(appClients)
	if strings.TrimSpace(token) == "" {
		token = envBootstrapToken()
	}
	return func(c fiber.Ctx) error {
		return cc.authenticateMCP(c, token)
	}
}

// MountStandalone registers the MCP streamable HTTP handler on a standalone Fiber app
// (typically listening on MCP_PORT). The handler is mounted at "/" so clients connect to
// http://host:MCP_PORT/ with the same API-key auth as /api/mcp.
func MountStandalone(app *fiber.App, appClients *config.AppClients, token string) error {
	if appClients == nil {
		return fmt.Errorf("mcp standalone: app clients unavailable")
	}
	if app == nil {
		return fmt.Errorf("mcp standalone: fiber app unavailable")
	}
	app.Use(tokenMiddleware(appClients, token))
	mountMCPHandler(app, newMCPServer(appClients))
	return nil
}

// newStandaloneApp builds and mounts an MCP Fiber app without listening.
func newStandaloneApp(appClients *config.AppClients, token string) (*fiber.App, error) {
	app := fiber.New(fiber.Config{
		AppName: "containerws-mcp",
	})
	if err := MountStandalone(app, appClients, token); err != nil {
		return nil, err
	}
	return app, nil
}

// ListenStandalone creates a Fiber app, mounts MCP, and serves it in a background goroutine.
// addr examples: ":9100", "0.0.0.0:9100". Returns the app so the caller can Shutdown.
func ListenStandalone(appClients *config.AppClients, addr, token string) (*fiber.App, error) {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return nil, fmt.Errorf("mcp standalone: listen address is required")
	}
	if !strings.Contains(addr, ":") {
		addr = net.JoinHostPort("", addr)
	}

	app, err := newStandaloneApp(appClients, token)
	if err != nil {
		return nil, err
	}

	go serveStandalone(app, addr)
	return app, nil
}

// serveStandalone blocks on Listen — always call via `go serveStandalone(...)`.
func serveStandalone(app *fiber.App, addr string) {
	log.Printf("MCP standalone listening on http://%s", normalizeListenLogAddr(addr))
	if err := app.Listen(addr, fiber.ListenConfig{DisableStartupMessage: true}); err != nil {
		log.Printf("MCP standalone stopped: %v", err)
	}
}

// StartStandaloneFromEnv starts a dedicated MCP listener when MCP_PORT is set.
// If MCP_PORT is missing, empty, or invalid, returns nil, nil and starts nothing.
// The HTTP server runs concurrently via `go serveStandalone`.
func StartStandaloneFromEnv(appClients *config.AppClients) (*fiber.App, error) {
	port := strings.TrimSpace(viper.GetString("MCP_PORT"))
	if !standalonePortConfigured(port) {
		return nil, nil
	}

	host := strings.TrimSpace(viper.GetString("MCP_ADDRESS"))
	if host == "" {
		if appClients != nil && appClients.ServerConfig() != nil && appClients.ServerConfig().ADDRESS != "" {
			host = appClients.ServerConfig().ADDRESS
		} else {
			host = "0.0.0.0"
		}
	}
	addr := net.JoinHostPort(host, port)
	token := strings.TrimSpace(viper.GetString("MCP_TOKEN"))
	return ListenStandalone(appClients, addr, token)
}

// RunStandaloneFromEnv runs MCP standalone on its own goroutine.
// Sends the Fiber app (or nil if disabled/error) on ready once, then blocks in Listen.
// Prefer this from the main server: go mcp.RunStandaloneFromEnv(appClients, ready)
func RunStandaloneFromEnv(appClients *config.AppClients, ready chan<- *fiber.App) {
	port := strings.TrimSpace(viper.GetString("MCP_PORT"))
	if !standalonePortConfigured(port) {
		if ready != nil {
			ready <- nil
		}
		return
	}

	host := strings.TrimSpace(viper.GetString("MCP_ADDRESS"))
	if host == "" {
		if appClients != nil && appClients.ServerConfig() != nil && appClients.ServerConfig().ADDRESS != "" {
			host = appClients.ServerConfig().ADDRESS
		} else {
			host = "0.0.0.0"
		}
	}
	addr := net.JoinHostPort(host, port)
	token := strings.TrimSpace(viper.GetString("MCP_TOKEN"))

	app, err := newStandaloneApp(appClients, token)
	if err != nil {
		log.Printf("MCP standalone: %v", err)
		if ready != nil {
			ready <- nil
		}
		return
	}
	if ready != nil {
		ready <- app
	}
	serveStandalone(app, addr)
}

// standalonePortConfigured is true only when MCP_PORT is a usable listen port.
func standalonePortConfigured(port string) bool {
	port = strings.TrimSpace(port)
	if port == "" || port == "0" {
		return false
	}
	n, err := net.LookupPort("tcp", port)
	if err != nil || n <= 0 {
		return false
	}
	return true
}

func normalizeListenLogAddr(addr string) string {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return addr
	}
	if host == "" || host == "0.0.0.0" {
		host = "127.0.0.1"
	}
	return net.JoinHostPort(host, port)
}
