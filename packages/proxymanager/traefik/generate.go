package traefik

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/izetmolla/containerws/models"
	"gopkg.in/yaml.v3"
)

// Generate writes static traefik.yml + dynamic file-provider config into configDir.
func Generate(settings *models.ProxySettings, hosts []models.ProxyHost, redirects []models.ProxyRedirect, certs []models.ProxyCertificate, configDir, appBaseURL string) (files []string, err error) {
	if settings == nil {
		return nil, fmt.Errorf("empty settings")
	}
	dynamicDir := filepath.Join(configDir, "dynamic")
	sslDir := filepath.Join(configDir, "ssl")
	_ = os.MkdirAll(dynamicDir, 0o755)
	_ = os.MkdirAll(sslDir, 0o755)

	httpPort := settings.HTTPPort
	httpsPort := settings.HTTPSPort
	staticPath := filepath.Join(configDir, "traefik.yml")

	providersFile := dynamicDir
	if settings.TraefikRuntime == models.ProxyRuntimeDocker {
		providersFile = "/etc/traefik/dynamic"
	}

	static := map[string]any{
		"api": map[string]any{
			"dashboard": false,
			"insecure":  false,
		},
		"entryPoints": map[string]any{
			"web": map[string]any{
				"address": fmt.Sprintf(":%d", httpPort),
			},
			"websecure": map[string]any{
				"address": fmt.Sprintf(":%d", httpsPort),
			},
		},
		"providers": map[string]any{
			"file": map[string]any{
				"directory": providersFile,
				"watch":     true,
			},
		},
		"log": map[string]any{
			"level": "INFO",
		},
	}
	staticBytes, err := yaml.Marshal(static)
	if err != nil {
		return nil, err
	}
	if err := writeFile(staticPath, staticBytes, 0o644); err != nil {
		return nil, err
	}
	files = append(files, staticPath)

	for i := range certs {
		c := &certs[i]
		if c.Source == models.ProxyCertPath {
			continue
		}
		if strings.TrimSpace(c.CertPEM) == "" || strings.TrimSpace(c.KeyPEM) == "" {
			continue
		}
		cp := filepath.Join(sslDir, c.ID+".crt")
		kp := filepath.Join(sslDir, c.ID+".key")
		if err := writeFile(cp, []byte(c.CertPEM), 0o644); err != nil {
			return files, err
		}
		if err := writeFile(kp, []byte(c.KeyPEM), 0o600); err != nil {
			return files, err
		}
		files = append(files, cp, kp)
	}

	dyn := buildDynamic(settings, hosts, redirects, certs, sslDir, appBaseURL)
	dynPath := filepath.Join(dynamicDir, "routes.yml")
	dynBytes, err := yaml.Marshal(dyn)
	if err != nil {
		return files, err
	}
	if err := writeFile(dynPath, dynBytes, 0o644); err != nil {
		return files, err
	}
	files = append(files, dynPath)
	return files, nil
}

func writeFile(path string, data []byte, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, mode); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func buildDynamic(settings *models.ProxySettings, hosts []models.ProxyHost, redirects []models.ProxyRedirect, certs []models.ProxyCertificate, sslDir, appBaseURL string) map[string]any {
	routers := map[string]any{}
	services := map[string]any{}
	middlewares := map[string]any{}
	tlsStores := map[string]any{}
	certificates := []any{}

	for i := range certs {
		c := &certs[i]
		var certFile, keyFile string
		if c.Source == models.ProxyCertPath {
			certFile, keyFile = c.CertPath, c.KeyPath
		} else if strings.TrimSpace(c.CertPEM) != "" {
			if settings.TraefikRuntime == models.ProxyRuntimeDocker {
				certFile = "/etc/traefik/ssl/" + c.ID + ".crt"
				keyFile = "/etc/traefik/ssl/" + c.ID + ".key"
			} else {
				certFile = filepath.Join(sslDir, c.ID+".crt")
				keyFile = filepath.Join(sslDir, c.ID+".key")
			}
		}
		if certFile != "" && keyFile != "" {
			certificates = append(certificates, map[string]any{
				"certFile": certFile,
				"keyFile":  keyFile,
			})
		}
	}
	if len(certificates) > 0 {
		tlsStores["default"] = map[string]any{
			"defaultCertificate": certificates[0],
		}
	}

	for i := range redirects {
		r := &redirects[i]
		name := fmt.Sprintf("redirect-%d", i)
		mw := name + "-mw"
		middlewares[mw] = map[string]any{
			"redirectRegex": map[string]any{
				"regex":       ".*",
				"replacement": r.ToURL,
				"permanent":   r.StatusCode == 301 || r.StatusCode == 308,
			},
		}
		rule := fmt.Sprintf("Host(`%s`)", r.FromHost)
		if r.FromPath != "/" {
			rule += fmt.Sprintf(" && PathPrefix(`%s`)", r.FromPath)
		}
		routers[name] = map[string]any{
			"rule":        rule,
			"entryPoints": []string{"web"},
			"middlewares": []string{mw},
			"service":     "noop@internal",
			"priority":    1000 + i,
		}
	}

	for i := range hosts {
		h := &hosts[i]
		svcName := fmt.Sprintf("svc-%d", i)
		up := resolveUpstream(h.UpstreamType, h.UpstreamTarget, appBaseURL)
		services[svcName] = map[string]any{
			"loadBalancer": map[string]any{
				"servers": []map[string]any{{"url": stripPath(up)}},
			},
		}
		hostRules := make([]string, 0, len(h.DomainList()))
		for _, d := range h.DomainList() {
			hostRules = append(hostRules, fmt.Sprintf("Host(`%s`)", d))
		}
		hostRule := strings.Join(hostRules, " || ")

		entryPoints := []string{}
		if h.ListenScheme == models.ProxySchemeHTTP || h.ListenScheme == models.ProxySchemeBoth {
			entryPoints = append(entryPoints, "web")
		}
		if h.ListenScheme == models.ProxySchemeHTTPS || h.ListenScheme == models.ProxySchemeBoth {
			entryPoints = append(entryPoints, "websecure")
		}
		if len(entryPoints) == 0 {
			entryPoints = []string{"web"}
		}

		if len(h.Locations) == 0 {
			rName := fmt.Sprintf("host-%d", i)
			router := map[string]any{
				"rule":        hostRule,
				"entryPoints": entryPoints,
				"service":     svcName,
				"priority":    100 + i,
			}
			if h.ListenScheme == models.ProxySchemeHTTPS || h.ListenScheme == models.ProxySchemeBoth {
				router["tls"] = map[string]any{}
			}
			routers[rName] = router
		} else {
			rName := fmt.Sprintf("host-%d-root", i)
			router := map[string]any{
				"rule":        hostRule,
				"entryPoints": entryPoints,
				"service":     svcName,
				"priority":    50 + i,
			}
			if h.ListenScheme == models.ProxySchemeHTTPS || h.ListenScheme == models.ProxySchemeBoth {
				router["tls"] = map[string]any{}
			}
			routers[rName] = router
			for j := range h.Locations {
				loc := &h.Locations[j]
				lsvc := fmt.Sprintf("svc-%d-loc-%d", i, j)
				lup := up
				if strings.TrimSpace(loc.UpstreamTarget) != "" {
					lup = resolveUpstream(loc.UpstreamType, loc.UpstreamTarget, appBaseURL)
				}
				services[lsvc] = map[string]any{
					"loadBalancer": map[string]any{
						"servers": []map[string]any{{"url": stripPath(lup)}},
					},
				}
				lrName := fmt.Sprintf("host-%d-loc-%d", i, j)
				rule := hostRule + fmt.Sprintf(" && PathPrefix(`%s`)", loc.PathPrefix)
				lr := map[string]any{
					"rule":        rule,
					"entryPoints": entryPoints,
					"service":     lsvc,
					"priority":    200 + j,
				}
				if loc.StripPrefix && loc.PathPrefix != "/" {
					mw := lrName + "-strip"
					middlewares[mw] = map[string]any{
						"stripPrefix": map[string]any{
							"prefixes": []string{loc.PathPrefix},
						},
					}
					lr["middlewares"] = []string{mw}
				}
				if h.ListenScheme == models.ProxySchemeHTTPS || h.ListenScheme == models.ProxySchemeBoth {
					lr["tls"] = map[string]any{}
				}
				routers[lrName] = lr
			}
		}
	}

	http := map[string]any{
		"routers":  routers,
		"services": services,
	}
	if len(middlewares) > 0 {
		http["middlewares"] = middlewares
	}
	out := map[string]any{"http": http}
	if len(certificates) > 0 {
		out["tls"] = map[string]any{
			"certificates": certificates,
			"stores":       tlsStores,
		}
	}
	return out
}

func resolveUpstream(kind, target, appBaseURL string) string {
	target = strings.TrimSpace(target)
	if kind == models.ProxyUpstreamAppPath {
		base := strings.TrimRight(strings.TrimSpace(appBaseURL), "/")
		if base == "" {
			base = "http://127.0.0.1"
		}
		if !strings.HasPrefix(target, "/") {
			target = "/" + target
		}
		return base + target
	}
	return target
}

func stripPath(u string) string {
	if i := strings.Index(u, "://"); i >= 0 {
		rest := u[i+3:]
		if j := strings.IndexByte(rest, '/'); j >= 0 {
			return u[:i+3+j]
		}
	}
	return u
}
