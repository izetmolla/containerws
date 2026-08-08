package proxymanager_test

import (
	"testing"

	"github.com/izetmolla/containerws/models"
	"github.com/izetmolla/containerws/packages/proxymanager"
)

func TestCheckComponentsFiber(t *testing.T) {
	s := models.NewDefaultProxySettings()
	s.ActiveEngine = models.ProxyEngineFiber
	check := proxymanager.CheckComponents(s, &proxymanager.RuntimeStatus{})
	if !check.Ready {
		t.Fatalf("fiber should be ready: %+v", check)
	}
}

func TestCheckComponentsNginxDockerMissing(t *testing.T) {
	s := models.NewDefaultProxySettings()
	s.ActiveEngine = models.ProxyEngineNginx
	s.NginxRuntime = models.ProxyRuntimeDocker
	check := proxymanager.CheckComponents(s, &proxymanager.RuntimeStatus{DockerAvailable: false, DockerError: "no sock"})
	if check.Ready {
		t.Fatal("expected not ready")
	}
	if len(check.Missing) == 0 {
		t.Fatal("expected missing docker")
	}
}

func TestCheckComponentsNginxDockerOK(t *testing.T) {
	s := models.NewDefaultProxySettings()
	s.ActiveEngine = models.ProxyEngineNginx
	s.NginxRuntime = models.ProxyRuntimeDocker
	check := proxymanager.CheckComponents(s, &proxymanager.RuntimeStatus{DockerAvailable: true})
	if !check.Ready || !check.DockerReady {
		t.Fatalf("expected ready: %+v", check)
	}
}

func TestCheckComponentsNginxHostMissingBinary(t *testing.T) {
	s := models.NewDefaultProxySettings()
	s.ActiveEngine = models.ProxyEngineNginx
	s.NginxRuntime = models.ProxyRuntimeHost
	check := proxymanager.CheckComponents(s, &proxymanager.RuntimeStatus{NginxInstalled: false})
	if check.Ready {
		t.Fatal("expected missing nginx")
	}
}
