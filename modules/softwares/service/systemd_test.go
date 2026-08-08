package service

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIsDockerUnit(t *testing.T) {
	cases := map[string]bool{
		"docker.service": true,
		"docker.socket":  true,
		"docker":         true,
		"Docker.Service": true,
		"nginx.service":  false,
		"":               false,
	}
	for in, want := range cases {
		if got := isDockerUnit(in); got != want {
			t.Fatalf("isDockerUnit(%q)=%v want %v", in, got, want)
		}
	}
}

func TestSummarize(t *testing.T) {
	if got := summarize([]UnitStatus{{Active: "active"}, {Active: "active"}}); got != "running" {
		t.Fatalf("got %s", got)
	}
	if got := summarize([]UnitStatus{{Active: "inactive"}, {Active: "inactive"}}); got != "stopped" {
		t.Fatalf("got %s", got)
	}
	if got := summarize([]UnitStatus{{Active: "unknown"}, {Active: "unknown"}}); got != "unavailable" {
		t.Fatalf("got %s", got)
	}
	if got := summarize([]UnitStatus{{Active: "active"}, {Active: "inactive"}}); got != "partial" {
		t.Fatalf("got %s", got)
	}
}

func TestProbeDockerDirectStopped(t *testing.T) {
	// Point at a missing socket so this host's real docker (if any) is ignored.
	t.Setenv("DOCKER_HOST", "unix:///tmp/containerws-no-such-docker.sock")
	st := probeDockerDirect("docker.service")
	if st.Active == "active" && !dockerdProcessRunning() {
		t.Fatalf("expected inactive without sock, got %+v", st)
	}
}

func TestProbeUnitsDockerFallbackWhenSystemdOffline(t *testing.T) {
	if systemdUsable() {
		t.Skip("systemd is usable on this host — skip direct-mode fallback test")
	}
	// Ensure sock exists only if docker is actually up; otherwise overall should be stopped.
	st := ProbeUnits([]string{"docker.service"})
	if !st.Managed {
		t.Fatal("expected managed")
	}
	if st.Overall == "unavailable" {
		t.Fatalf("docker should not be unavailable in direct mode; got %+v", st)
	}
	if dockerEngineReachable() && st.Overall != "running" {
		t.Fatalf("reachable docker should be running, got %s (%+v)", st.Overall, st.Units)
	}
	if !dockerEngineReachable() && !dockerdProcessRunning() && st.Overall != "stopped" {
		t.Fatalf("down docker should be stopped, got %s", st.Overall)
	}
}

func TestDockerSockPathEnv(t *testing.T) {
	dir := t.TempDir()
	sock := filepath.Join(dir, "docker.sock")
	t.Setenv("DOCKER_HOST", "unix://"+sock)
	if got := dockerSockPath(); got != sock {
		t.Fatalf("got %q want %q", got, sock)
	}
	_ = os.Remove(sock)
}
