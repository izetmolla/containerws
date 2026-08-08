package dockerrun_test

import (
	"testing"

	"github.com/izetmolla/containerws/models"
	"github.com/izetmolla/containerws/packages/proxymanager/dockerrun"
)

func TestBuildPlanPublishedBindIP(t *testing.T) {
	s := &models.ProxySettings{
		HTTPPort:          8080,
		HTTPSPort:         8443,
		DockerNetworkMode: models.ProxyDockerNetPublished,
		DockerPublishIP:   "10.0.0.5",
	}
	s.Normalize()
	plan, err := dockerrun.BuildPlan(s, "80", "443")
	if err != nil {
		t.Fatal(err)
	}
	if plan.NetworkMode != "" {
		t.Fatalf("expected empty network mode, got %q", plan.NetworkMode)
	}
	b80 := plan.PortBindings["80/tcp"]
	if len(b80) != 1 || b80[0].HostIP != "10.0.0.5" || b80[0].HostPort != "8080" {
		t.Fatalf("unexpected http binding: %+v", b80)
	}
}

func TestBuildPlanHost(t *testing.T) {
	s := &models.ProxySettings{DockerNetworkMode: models.ProxyDockerNetHost}
	s.Normalize()
	plan, err := dockerrun.BuildPlan(s, "80", "443")
	if err != nil {
		t.Fatal(err)
	}
	if plan.NetworkMode != "host" {
		t.Fatalf("mode=%q", plan.NetworkMode)
	}
	if plan.PortBindings != nil {
		t.Fatal("host mode should not publish ports")
	}
}

func TestBuildPlanMacvlanRequiresNetwork(t *testing.T) {
	s := &models.ProxySettings{DockerNetworkMode: models.ProxyDockerNetMacvlan}
	s.Normalize()
	if _, err := dockerrun.BuildPlan(s, "80", "443"); err == nil {
		t.Fatal("expected error without network name")
	}
	s.DockerNetworkName = "lan"
	s.DockerIPv4Address = "192.168.1.50"
	plan, err := dockerrun.BuildPlan(s, "80", "443")
	if err != nil {
		t.Fatal(err)
	}
	if plan.NetworkMode != "lan" {
		t.Fatalf("mode=%q", plan.NetworkMode)
	}
	ep := plan.NetworkingConfig.EndpointsConfig["lan"]
	if ep == nil || ep.IPAMConfig == nil || ep.IPAMConfig.IPv4Address != "192.168.1.50" {
		t.Fatalf("unexpected endpoint: %+v", ep)
	}
}
