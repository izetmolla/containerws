package dockerrun

import (
	"fmt"
	"strings"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/go-connections/nat"
	"github.com/izetmolla/containerws/models"
)

// Plan describes Docker HostConfig networking pieces for the proxy container.
type Plan struct {
	ExposedPorts    nat.PortSet
	PortBindings    nat.PortMap
	NetworkMode     container.NetworkMode
	NetworkingConfig *network.NetworkingConfig
}

// BuildPlan creates port publish / host / macvlan networking from proxy settings.
// containerHTTP/HTTPS are the ports the process listens on inside the container.
func BuildPlan(settings *models.ProxySettings, containerHTTP, containerHTTPS string) (*Plan, error) {
	if settings == nil {
		return nil, fmt.Errorf("empty settings")
	}
	mode := settings.DockerNetworkMode
	if mode == "" {
		mode = models.ProxyDockerNetPublished
	}

	switch mode {
	case models.ProxyDockerNetHost:
		return &Plan{
			NetworkMode: container.NetworkMode("host"),
		}, nil

	case models.ProxyDockerNetMacvlan:
		netName := strings.TrimSpace(settings.DockerNetworkName)
		if netName == "" {
			return nil, fmt.Errorf("docker_network_name is required for macvlan / dedicated-IP mode")
		}
		ep := &network.EndpointSettings{}
		if ip := strings.TrimSpace(settings.DockerIPv4Address); ip != "" {
			ep.IPAMConfig = &network.EndpointIPAMConfig{IPv4Address: ip}
		}
		return &Plan{
			NetworkMode: container.NetworkMode(netName),
			NetworkingConfig: &network.NetworkingConfig{
				EndpointsConfig: map[string]*network.EndpointSettings{
					netName: ep,
				},
			},
			// Still expose for documentation; no host publish.
			ExposedPorts: nat.PortSet{
				nat.Port(containerHTTP + "/tcp"):  struct{}{},
				nat.Port(containerHTTPS + "/tcp"): struct{}{},
			},
		}, nil

	default: // published
		hostIP := strings.TrimSpace(settings.DockerPublishIP)
		if hostIP == "" {
			hostIP = "0.0.0.0"
		}
		httpHost := fmt.Sprintf("%d", settings.HTTPPort)
		httpsHost := fmt.Sprintf("%d", settings.HTTPSPort)
		exposed := nat.PortSet{
			nat.Port(containerHTTP + "/tcp"):  struct{}{},
			nat.Port(containerHTTPS + "/tcp"): struct{}{},
		}
		bindings := nat.PortMap{
			nat.Port(containerHTTP + "/tcp"): []nat.PortBinding{
				{HostIP: hostIP, HostPort: httpHost},
			},
			nat.Port(containerHTTPS + "/tcp"): []nat.PortBinding{
				{HostIP: hostIP, HostPort: httpsHost},
			},
		}
		return &Plan{
			ExposedPorts: exposed,
			PortBindings: bindings,
		}, nil
	}
}

// ApplyToHostConfig sets NetworkMode and PortBindings on an existing HostConfig.
func (p *Plan) ApplyToHostConfig(host *container.HostConfig) {
	if p == nil || host == nil {
		return
	}
	if p.NetworkMode != "" {
		host.NetworkMode = p.NetworkMode
	}
	if p.PortBindings != nil {
		host.PortBindings = p.PortBindings
	}
}
