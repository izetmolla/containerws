package proxymanager

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/izetmolla/containerws/models"
)

// ValidateSnapshot checks settings + hosts before apply.
func ValidateSnapshot(snap *Snapshot) error {
	if snap == nil || snap.Settings == nil {
		return fmt.Errorf("empty snapshot")
	}
	s := snap.Settings
	switch s.ActiveEngine {
	case models.ProxyEngineFiber, models.ProxyEngineNginx, models.ProxyEngineTraefik:
	default:
		return fmt.Errorf("unsupported active_engine %q", s.ActiveEngine)
	}
	if s.HTTPPort <= 0 || s.HTTPPort > 65535 {
		return fmt.Errorf("invalid http_port %d", s.HTTPPort)
	}
	if s.HTTPSPort <= 0 || s.HTTPSPort > 65535 {
		return fmt.Errorf("invalid https_port %d", s.HTTPSPort)
	}
	if (s.ActiveEngine == models.ProxyEngineNginx && s.NginxRuntime == models.ProxyRuntimeDocker) ||
		(s.ActiveEngine == models.ProxyEngineTraefik && s.TraefikRuntime == models.ProxyRuntimeDocker) {
		switch s.DockerNetworkMode {
		case models.ProxyDockerNetPublished, models.ProxyDockerNetHost, "", "bridge":
		case models.ProxyDockerNetMacvlan:
			if strings.TrimSpace(s.DockerNetworkName) == "" {
				return fmt.Errorf("docker_network_name is required when docker_network_mode=macvlan")
			}
		default:
			return fmt.Errorf("unsupported docker_network_mode %q", s.DockerNetworkMode)
		}
	}
	for i := range snap.Hosts {
		h := &snap.Hosts[i]
		if len(h.DomainList()) == 0 {
			return fmt.Errorf("host %q has no domains", h.Name)
		}
		if strings.TrimSpace(h.UpstreamTarget) == "" {
			return fmt.Errorf("host %q has empty upstream", h.Name)
		}
		if h.UpstreamType == models.ProxyUpstreamURL {
			if _, err := url.ParseRequestURI(h.UpstreamTarget); err != nil {
				// Allow host:port style after Normalize builds scheme://host:port
				if h.ForwardHost == "" {
					return fmt.Errorf("host %q upstream url: %w", h.Name, err)
				}
			}
		}
		if h.ListenScheme == models.ProxySchemeHTTPS || h.ListenScheme == models.ProxySchemeBoth {
			if h.CertificateID == nil || strings.TrimSpace(*h.CertificateID) == "" {
				return fmt.Errorf("host %q requires a certificate for HTTPS", h.Name)
			}
			if snap.CertByID(*h.CertificateID) == nil {
				return fmt.Errorf("host %q certificate not found", h.Name)
			}
		}
		for j := range h.Locations {
			loc := &h.Locations[j]
			if loc.UpstreamTarget != "" && loc.UpstreamType == models.ProxyUpstreamURL {
				if _, err := url.ParseRequestURI(loc.UpstreamTarget); err != nil {
					return fmt.Errorf("host %q location %q upstream: %w", h.Name, loc.PathPrefix, err)
				}
			}
		}
	}
	for i := range snap.Redirects {
		r := &snap.Redirects[i]
		if strings.TrimSpace(r.FromHost) == "" {
			return fmt.Errorf("redirect %q missing from_host", r.Name)
		}
		if strings.TrimSpace(r.ToURL) == "" {
			return fmt.Errorf("redirect %q missing to_url", r.Name)
		}
	}
	return nil
}
