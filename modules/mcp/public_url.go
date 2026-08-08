package mcp

import (
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/izetmolla/containerws/config"
	"github.com/izetmolla/containerws/packages/machine"
	"github.com/spf13/viper"
)

// resolveAuthURL returns AUTH_URL from process env, viper, server config, or environments.
func resolveAuthURL(appClients *config.AppClients) string {
	if v := strings.TrimSpace(os.Getenv("AUTH_URL")); v != "" {
		return v
	}
	if v := strings.TrimSpace(viper.GetString("AUTH_URL")); v != "" {
		return v
	}
	if appClients != nil {
		if sc := appClients.ServerConfig(); sc != nil {
			if v := strings.TrimSpace(sc.AUTH_URL); v != "" {
				return v
			}
		}
		if env := appClients.Environments(); env != nil {
			if v := strings.TrimSpace(env.Get("AUTH_URL")); v != "" {
				return v
			}
		}
	}
	return ""
}

func httpsEnabled(appClients *config.AppClients) bool {
	raw := strings.TrimSpace(os.Getenv("ENABLE_HTTPS"))
	if raw == "" {
		raw = strings.TrimSpace(viper.GetString("ENABLE_HTTPS"))
	}
	if raw != "" {
		return config.ParseBoolEnv(raw)
	}
	if appClients != nil {
		if sc := appClients.ServerConfig(); sc != nil && sc.ENABLE_HTTPS {
			return true
		}
		if env := appClients.Environments(); env != nil {
			return config.ParseBoolEnv(env.Get("ENABLE_HTTPS"))
		}
	}
	return false
}

func mainListenPort(appClients *config.AppClients) string {
	if appClients != nil && appClients.ServerConfig() != nil {
		if p := strings.TrimSpace(appClients.ServerConfig().PORT); p != "" {
			return p
		}
	}
	if p := strings.TrimSpace(os.Getenv("PORT")); p != "" {
		return p
	}
	if p := strings.TrimSpace(viper.GetString("PORT")); p != "" {
		return p
	}
	return config.DefaultHTTPPort
}

// publicHostForBind picks the hostname clients should use for MCP.
// Prefer AUTH_URL host, then a specific bind address, then primary/app IP.
func publicHostForBind(appClients *config.AppClients, bindAddress string) (scheme, host string) {
	scheme = "http"
	if httpsEnabled(appClients) {
		scheme = "https"
	}

	if auth := resolveAuthURL(appClients); auth != "" {
		if u, err := url.Parse(auth); err == nil {
			if h := strings.TrimSpace(u.Hostname()); h != "" {
				if u.Scheme != "" {
					scheme = u.Scheme
				}
				return scheme, h
			}
		}
	}

	bind := strings.TrimSpace(bindAddress)
	if bind != "" && bind != "0.0.0.0" && bind != "::" && bind != "[::]" {
		return scheme, bind
	}

	if appClients != nil {
		if sc := appClients.ServerConfig(); sc != nil {
			if a := strings.TrimSpace(sc.ADDRESS); a != "" && a != "0.0.0.0" && a != "::" {
				return scheme, a
			}
		}
		if env := appClients.Environments(); env != nil {
			if a := strings.TrimSpace(env.Get("ADDRESS")); a != "" && a != "0.0.0.0" && a != "::" {
				return scheme, a
			}
		}
	}

	ip := strings.TrimSpace(machine.Detect().PrimaryIP)
	if ip == "" {
		ip = "127.0.0.1"
	}
	return scheme, ip
}

func standalonePublicURL(appClients *config.AppClients, bindAddress string, port int) string {
	if port <= 0 {
		return ""
	}
	scheme, host := publicHostForBind(appClients, bindAddress)
	return scheme + "://" + net.JoinHostPort(host, strconv.Itoa(port))
}

func mainAPIMCPURL(appClients *config.AppClients) string {
	if auth := resolveAuthURL(appClients); auth != "" {
		return strings.TrimRight(auth, "/") + "/api/mcp"
	}
	scheme, host := publicHostForBind(appClients, "")
	return scheme + "://" + net.JoinHostPort(host, mainListenPort(appClients)) + "/api/mcp"
}
