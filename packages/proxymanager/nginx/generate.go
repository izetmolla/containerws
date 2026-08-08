package nginx

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/izetmolla/containerws/models"
)

// Generate writes nginx.conf + conf.d host files + SSL material into configDir.
func Generate(settings *models.ProxySettings, hosts []models.ProxyHost, redirects []models.ProxyRedirect, certs []models.ProxyCertificate, configDir, appBaseURL string) (files []string, err error) {
	if settings == nil {
		return nil, fmt.Errorf("empty settings")
	}
	confd := filepath.Join(configDir, "conf.d")
	sslDir := filepath.Join(configDir, "ssl")
	_ = os.MkdirAll(confd, 0o755)
	_ = os.MkdirAll(sslDir, 0o755)

	main := filepath.Join(configDir, "nginx.conf")
	mainBody := fmt.Sprintf(`worker_processes auto;
error_log /var/log/nginx/error.log warn;
pid /var/run/nginx.pid;

events {
    worker_connections 1024;
}

http {
    include       /etc/nginx/mime.types;
    default_type  application/octet-stream;
    sendfile        on;
    keepalive_timeout  65;
    map $http_upgrade $connection_upgrade {
        default upgrade;
        ''      close;
    }
    include %s/*.conf;
}
`, confd)
	if settings.NginxRuntime == models.ProxyRuntimeDocker {
		mainBody = `worker_processes auto;
error_log /var/log/nginx/error.log warn;
pid /var/run/nginx.pid;

events {
    worker_connections 1024;
}

http {
    include       /etc/nginx/mime.types;
    default_type  application/octet-stream;
    sendfile        on;
    keepalive_timeout  65;
    map $http_upgrade $connection_upgrade {
        default upgrade;
        ''      close;
    }
    include /etc/nginx/conf.d/*.conf;
}
`
	}
	if err := writeFile(main, []byte(mainBody), 0o644); err != nil {
		return nil, err
	}
	files = append(files, main)

	certByID := map[string]*models.ProxyCertificate{}
	for i := range certs {
		certByID[certs[i].ID] = &certs[i]
	}

	certFiles, err := writeCertificates(certs, sslDir)
	if err != nil {
		return files, err
	}
	files = append(files, certFiles...)

	entries, _ := os.ReadDir(confd)
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".conf") {
			_ = os.Remove(filepath.Join(confd, e.Name()))
		}
	}

	for i := range redirects {
		r := &redirects[i]
		name := fmt.Sprintf("redirect-%03d-%s.conf", i, sanitize(r.FromHost))
		path := filepath.Join(confd, name)
		httpPort, _ := settings.NginxContainerListenPorts()
		if settings.NginxRuntime == models.ProxyRuntimeHost {
			httpPort = settings.HTTPPort
		}
		body := renderRedirect(r, httpPort)
		if err := writeFile(path, []byte(body), 0o644); err != nil {
			return files, err
		}
		files = append(files, path)
	}

	for i := range hosts {
		h := &hosts[i]
		name := fmt.Sprintf("host-%03d-%s.conf", i, sanitize(h.Name))
		path := filepath.Join(confd, name)
		body := renderHost(settings, h, certByID, sslDir, appBaseURL)
		if err := writeFile(path, []byte(body), 0o644); err != nil {
			return files, err
		}
		files = append(files, path)
	}

	if len(hosts) == 0 && len(redirects) == 0 {
		def := filepath.Join(confd, "00-default.conf")
		httpPort, _ := settings.NginxContainerListenPorts()
		if settings.NginxRuntime == models.ProxyRuntimeHost {
			httpPort = settings.HTTPPort
		}
		body := fmt.Sprintf(`server {
    listen %d default_server;
    server_name _;
    location / {
        return 200 'containerws proxy manager (nginx) — no hosts configured\n';
        add_header Content-Type text/plain;
    }
}
`, httpPort)
		if err := writeFile(def, []byte(body), 0o644); err != nil {
			return files, err
		}
		files = append(files, def)
	}
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

func writeCertificates(certs []models.ProxyCertificate, sslDir string) ([]string, error) {
	var files []string
	for i := range certs {
		c := &certs[i]
		certPath := filepath.Join(sslDir, c.ID+".crt")
		keyPath := filepath.Join(sslDir, c.ID+".key")
		if c.Source == models.ProxyCertPath {
			continue
		}
		if strings.TrimSpace(c.CertPEM) == "" || strings.TrimSpace(c.KeyPEM) == "" {
			continue
		}
		if err := writeFile(certPath, []byte(c.CertPEM), 0o644); err != nil {
			return files, err
		}
		if err := writeFile(keyPath, []byte(c.KeyPEM), 0o600); err != nil {
			return files, err
		}
		files = append(files, certPath, keyPath)
	}
	return files, nil
}

func certPaths(settings *models.ProxySettings, h *models.ProxyHost, certByID map[string]*models.ProxyCertificate, sslDir string) (cert, key string) {
	if h.CertificateID == nil {
		return "", ""
	}
	c := certByID[*h.CertificateID]
	if c == nil {
		return "", ""
	}
	if c.Source == models.ProxyCertPath {
		return c.CertPath, c.KeyPath
	}
	if settings.NginxRuntime == models.ProxyRuntimeDocker {
		return "/etc/nginx/ssl/" + c.ID + ".crt", "/etc/nginx/ssl/" + c.ID + ".key"
	}
	return filepath.Join(sslDir, c.ID+".crt"), filepath.Join(sslDir, c.ID+".key")
}

func renderRedirect(r *models.ProxyRedirect, httpPort int) string {
	to := r.ToURL
	if r.PreservePath {
		to = strings.TrimRight(to, "/") + "$request_uri"
	}
	return fmt.Sprintf(`server {
    listen %d;
    server_name %s;
    location %s {
        return %d %s;
    }
}
`, httpPort, r.FromHost, r.FromPath, r.StatusCode, to)
}

func renderHost(settings *models.ProxySettings, h *models.ProxyHost, certByID map[string]*models.ProxyCertificate, sslDir, appBaseURL string) string {
	var b strings.Builder
	domains := strings.Join(h.DomainList(), " ")
	upstream := resolveUpstream(h.UpstreamType, h.UpstreamTarget, appBaseURL)
	listenHTTPPort, listenHTTPSPort := settings.NginxContainerListenPorts()
	if settings.NginxRuntime == models.ProxyRuntimeHost {
		listenHTTPPort, listenHTTPSPort = settings.HTTPPort, settings.HTTPSPort
	}
	listenHTTP := h.ListenScheme == models.ProxySchemeHTTP || h.ListenScheme == models.ProxySchemeBoth || h.SSLForced
	listenHTTPS := h.ListenScheme == models.ProxySchemeHTTPS || h.ListenScheme == models.ProxySchemeBoth || h.SSLForced

	writeServer := func(ssl bool) {
		b.WriteString("server {\n")
		if ssl {
			if h.HTTP2Support {
				fmt.Fprintf(&b, "    listen %d ssl http2;\n", listenHTTPSPort)
			} else {
				fmt.Fprintf(&b, "    listen %d ssl;\n", listenHTTPSPort)
			}
			cert, key := certPaths(settings, h, certByID, sslDir)
			fmt.Fprintf(&b, "    ssl_certificate     %s;\n", cert)
			fmt.Fprintf(&b, "    ssl_certificate_key %s;\n", key)
		} else {
			fmt.Fprintf(&b, "    listen %d;\n", listenHTTPPort)
			if h.SSLForced && listenHTTPS {
				b.WriteString("    return 301 https://$host$request_uri;\n")
				b.WriteString("}\n\n")
				return
			}
		}
		fmt.Fprintf(&b, "    server_name %s;\n", domains)

		if h.BlockExploits {
			b.WriteString(`    location ~* /(\.|wp-config\.php|xmlrpc\.php|wp-login\.php) {
        deny all;
        return 404;
    }
`)
		}

		writeLocation(&b, "/", upstream, false, h.Websocket, h.CachingEnabled, h.CustomHeaders)
		for j := range h.Locations {
			loc := &h.Locations[j]
			up := upstream
			if strings.TrimSpace(loc.UpstreamTarget) != "" {
				up = resolveUpstream(loc.UpstreamType, loc.UpstreamTarget, appBaseURL)
			}
			writeLocation(&b, loc.PathPrefix, up, loc.StripPrefix, loc.Websocket || h.Websocket, h.CachingEnabled, h.CustomHeaders)
		}
		b.WriteString("}\n\n")
	}

	if listenHTTP {
		writeServer(false)
	}
	if listenHTTPS {
		writeServer(true)
	}
	return b.String()
}

func writeLocation(b *strings.Builder, prefix, upstream string, strip, ws, cache bool, headers models.JSONBAny) {
	fmt.Fprintf(b, "    location %s {\n", prefix)
	if strip && prefix != "/" {
		fmt.Fprintf(b, "        rewrite ^%s/?(.*)$ /$1 break;\n", strings.TrimRight(prefix, "/"))
	}
	proxyPass := strings.TrimRight(upstream, "/")
	fmt.Fprintf(b, "        proxy_pass %s;\n", proxyPass)
	b.WriteString("        proxy_set_header Host $host;\n")
	b.WriteString("        proxy_set_header X-Real-IP $remote_addr;\n")
	b.WriteString("        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;\n")
	b.WriteString("        proxy_set_header X-Forwarded-Proto $scheme;\n")
	if ws {
		b.WriteString("        proxy_http_version 1.1;\n")
		b.WriteString("        proxy_set_header Upgrade $http_upgrade;\n")
		b.WriteString("        proxy_set_header Connection $connection_upgrade;\n")
	}
	if cache {
		b.WriteString("        proxy_cache_valid 200 1h;\n")
		b.WriteString("        add_header X-Cache-Status $upstream_cache_status;\n")
	}
	for k, v := range headers {
		if s, ok := v.(string); ok {
			fmt.Fprintf(b, "        proxy_set_header %s %s;\n", k, s)
		}
	}
	b.WriteString("    }\n")
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

func sanitize(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			b.WriteRune(r)
		} else {
			b.WriteByte('-')
		}
	}
	out := b.String()
	if out == "" {
		return "host"
	}
	if len(out) > 40 {
		out = out[:40]
	}
	return out
}
