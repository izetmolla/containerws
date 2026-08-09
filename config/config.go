package config

import (
	"context"
	"errors"
	"log"
	"os"
	"path/filepath"

	"github.com/izetmolla/containerws/frontend"
	"github.com/izetmolla/containerws/models"
	"github.com/izetmolla/containerws/packages/environments"
	"github.com/izetmolla/containerws/packages/machine"
	"github.com/izetmolla/containerws/packages/render"
	"github.com/izetmolla/goauth"
	"github.com/izetmolla/goauthfiberv3"
	"github.com/spf13/viper"
	"gorm.io/gorm"
)

type AppClients struct {
	serverConfig  *ServerConfig
	db            *gorm.DB
	render        *render.Render
	authorization *goauthfiberv3.Auth
	environments  *environments.Environments
}

type ConfigSettings struct {
	Port        string
	Address     string
	JWTSecret   string
	DatabaseURL string
	AuthURL     string
}

func BootApplication() (*AppClients, error) {
	defaultServerConfig := &ServerConfig{
		PORT:    DefaultHTTPPort,
		ADDRESS: "0.0.0.0",
	}
	var err error
	app := AppClients{
		serverConfig: defaultServerConfig,
	}
	// Process env (e.g. Kubernetes env / secrets) must work without a .env file.
	viper.AutomaticEnv()
	if _, statErr := os.Stat(".env"); statErr == nil {
		viper.SetConfigFile(".env")
		if err := viper.ReadInConfig(); err != nil {
			return &app, err
		}
	} else if !errors.Is(statErr, os.ErrNotExist) {
		return &app, statErr
	}
	if viper.GetString("ENV") == "development" {
		os.Setenv("ENV", "development")
	} else {
		os.Setenv("ENV", "production")
	}
	defaultServerConfig.SSL_DIR = ResolveSSLDir()
	databaseURL := viper.GetString("DATABASE_URL")
	if databaseURL == "" {
		if viper.GetString("ENV") == "development" {
			databaseURL = "./tmp/database.sqlite"
		} else {
			os.MkdirAll("/config/containerws/database", 0755)
			databaseURL = filepath.Join("/config/containerws/database", "database.sqlite")
		}
		os.Setenv("DATABASE_URL", databaseURL)
	}
	if abs, absErr := filepath.Abs(databaseURL); absErr == nil {
		databaseURL = abs
	}

	if viper.GetBool("start") {
		log.Printf("sqlite database: %s", databaseURL)
	}

	if app.db, err = InitializeDatabase(databaseURL); err != nil {
		return &app, errors.New("failed to initialize database: " + err.Error())
	}

	if err = AutoMigrateSQLite(app.db, models.AllModels()...); err != nil {
		return &app, errors.New("failed to migrate database: " + err.Error())
	}

	if err = models.EnsureModuleSidebarDefaults(app.db); err != nil {
		return &app, errors.New("failed to seed module sidebar defaults: " + err.Error())
	}

	if _, syncErr := machine.SyncCurrentContainer(context.Background(), app.db); syncErr != nil {
		log.Printf("container sync failed: %v", syncErr)
	}

	app.environments, err = environments.New(
		environments.NewConfig(app.db, nil).
			WithShellOverrides(environments.BuildShellOverrides(viper.GetString)).
			WithModuleID(app.ModuleName()),
	)
	if err != nil {
		return &app, errors.New("failed to initialize environments: " + err.Error())
	}

	server, err := app.GetServerConfigData(app.environments)
	if err != nil {
		return &app, errors.New("failed to get server config: " + err.Error())
	}

	authorization, err := goauth.New(&goauth.Config{
		JWTSecret: server.JWT_SECRET,
		AuthURL:   server.AUTH_URL,
		DB:        app.db,
	})
	if err != nil {
		return &app, errors.New("failed to initialize authorization: " + err.Error())
	}
	app.authorization = goauthfiberv3.New(authorization)

	app.environments.SetHooks(environments.Hooks{
		OnReload: app.onEnvironmentReload,
	})
	if err := app.environments.Reload(context.Background()); err != nil {
		log.Printf("environment reload after hook registration failed: %v", err)
	}

	app.render = render.New().
		WithModuleName(app.ModuleName()).
		WithAssets(frontend.GetStatic()).
		WithDB(app.db)
	return &app, err
}

func (a *AppClients) onEnvironmentReload(cfg environments.ServerConfig) {
	if a.serverConfig == nil {
		a.serverConfig = &ServerConfig{}
	}
	if cfg.PORT != "" {
		a.serverConfig.PORT = cfg.PORT
	}
	if cfg.ADDRESS != "" {
		a.serverConfig.ADDRESS = cfg.ADDRESS
	}
	if cfg.AUTH_URL != "" {
		a.serverConfig.AUTH_URL = cfg.AUTH_URL
	}
	if cfg.JWT_SECRET != "" {
		a.serverConfig.JWT_SECRET = cfg.JWT_SECRET
	}
	if cfg.ENABLE_HTTPS != "" {
		a.serverConfig.ENABLE_HTTPS = ParseBoolEnv(cfg.ENABLE_HTTPS)
	}
	if cfg.SERVER_SSL_CERTIFICATE != "" {
		a.serverConfig.SSLCertPEM = cfg.SERVER_SSL_CERTIFICATE
	}
	if cfg.SERVER_SSL_KEY != "" {
		a.serverConfig.SSLKeyPEM = cfg.SERVER_SSL_KEY
	}
	if a.authorization != nil {
		a.authorization.WithJWTSecret(cfg.JWT_SECRET).WithAuthURL(cfg.AUTH_URL)
	}
}

func (a *AppClients) ServerConfig() *ServerConfig {
	return a.serverConfig
}
func (a *AppClients) Authorization() *goauthfiberv3.Auth {
	return a.authorization
}

func (a *AppClients) Environments() *environments.Environments {
	return a.environments
}

// freshUserRoles returns the latest roles from the users table, falling back to token/session roles.
func (app *AppClients) FreshUserRoles(ctx context.Context, userID string, fallback []string) []string {
	if userID == "" || app.db == nil {
		return fallback
	}

	user, err := gorm.G[models.User](app.db).Select("roles").Where("id = ?", userID).First(ctx)
	if err != nil {
		return fallback
	}
	out := make([]string, 0, len(user.Roles))
	for _, r := range user.Roles {
		if s, ok := r.(string); ok && s != "" {
			out = append(out, s)
		}
	}
	return out
}

// ModuleName identifies this application for module-scoped environments and render data.
func (a *AppClients) ModuleName() string {
	return "containerws"
}
