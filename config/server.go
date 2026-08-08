package config

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"strings"

	"github.com/izetmolla/containerws/packages/environments"
	"github.com/spf13/viper"
)

type ServerConfig struct {
	PORT       string
	ADDRESS    string
	AUTH_URL   string
	JWT_SECRET string

	ENABLE_HTTPS bool
	SSLCertPEM   string
	SSLKeyPEM    string
	SSL_DIR      string
}

func (a *AppClients) GetServerConfigData(envMgr *environments.Environments) (*ServerConfig, error) {
	ctx := context.Background()
	secret, _ := a.RandomString(32)

	core, err := envMgr.ServerConfigFromDB(ctx, map[string]string{
		"PORT":                   viper.GetString("PORT"),
		"ADDRESS":                viper.GetString("ADDRESS"),
		"AUTH_URL":               viper.GetString("AUTH_URL"),
		"JWT_SECRET":             viper.GetString("JWT_SECRET"),
		"ENABLE_HTTPS":           viper.GetString("ENABLE_HTTPS"),
		"SERVER_SSL_CERTIFICATE": viper.GetString("SERVER_SSL_CERTIFICATE"),
		"SERVER_SSL_KEY":         viper.GetString("SERVER_SSL_KEY"),
	}, map[string]string{
		"PORT":                   DefaultHTTPPort,
		"ADDRESS":                "0.0.0.0",
		"AUTH_URL":               "",
		"JWT_SECRET":             secret,
		"ENABLE_HTTPS":           "false",
		"SERVER_SSL_CERTIFICATE": "",
		"SERVER_SSL_KEY":         "",
	})
	if err != nil {
		return nil, err
	}

	cfg := &ServerConfig{
		PORT:         core.PORT,
		ADDRESS:      core.ADDRESS,
		AUTH_URL:     core.AUTH_URL,
		JWT_SECRET:   core.JWT_SECRET,
		ENABLE_HTTPS: ParseBoolEnv(core.ENABLE_HTTPS),
		SSLCertPEM:   core.SERVER_SSL_CERTIFICATE,
		SSLKeyPEM:    core.SERVER_SSL_KEY,
		SSL_DIR:      ResolveSSLDir(),
	}
	a.serverConfig = cfg
	return cfg, nil
}

// ParseBoolEnv treats common truthy strings as true (case-insensitive).
func ParseBoolEnv(raw string) bool {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "1", "true", "yes", "on", "y", "t":
		return true
	default:
		return false
	}
}

func (a *AppClients) RandomString(size int) (string, error) {
	b := make([]byte, size)

	if _, err := rand.Read(b); err != nil {
		return "", err
	}

	return hex.EncodeToString(b), nil
}
