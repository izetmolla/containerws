package setup

import (
	"errors"
	"slices"
	"strings"
	"testing"
)

func TestResolveFamily(t *testing.T) {
	t.Parallel()

	tests := []struct {
		id, name string
		want     PackageFamily
		wantMgr  string
	}{
		{"ubuntu", "Ubuntu", FamilyDebian, "apt-get"},
		{"debian", "Debian GNU/Linux", FamilyDebian, "apt-get"},
		{"linuxmint", "Linux Mint", FamilyDebian, "apt-get"},
		{"pop", "Pop!_OS", FamilyDebian, "apt-get"},
		{"arch", "Arch Linux", FamilyArch, "pacman"},
		{"manjaro", "Manjaro Linux", FamilyArch, "pacman"},
		{"", "Ubuntu 26.04 LTS", FamilyDebian, "apt-get"},
		{"", "Something Unknown", FamilyUnknown, ""},
		{"void", "Void Linux", FamilyUnknown, ""},
	}

	for _, tc := range tests {
		t.Run(tc.id+"/"+tc.name, func(t *testing.T) {
			t.Parallel()
			got, mgr := resolveFamily(tc.id, tc.name)
			if got != tc.want {
				t.Fatalf("family: got %q want %q", got, tc.want)
			}
			if tc.want == FamilyRHEL {
				// dnf vs yum depends on host; just ensure a manager is set.
				if mgr != "dnf" && mgr != "yum" {
					t.Fatalf("rhel manager: got %q", mgr)
				}
				return
			}
			if mgr != tc.wantMgr {
				t.Fatalf("manager: got %q want %q", mgr, tc.wantMgr)
			}
		})
	}

	// RHEL family by id (manager depends on host binaries).
	fam, mgr := resolveFamily("fedora", "Fedora Linux")
	if fam != FamilyRHEL {
		t.Fatalf("fedora family: got %q", fam)
	}
	if mgr != "dnf" && mgr != "yum" {
		t.Fatalf("fedora manager: got %q", mgr)
	}
}

func TestVersionMajor(t *testing.T) {
	t.Parallel()
	cases := map[string]int{
		"":        0,
		"26.04":   26,
		"24.04":   24,
		"12":      12,
		"9.5":     9,
		"rolling": 0,
	}
	for in, want := range cases {
		if got := versionMajor(in); got != want {
			t.Errorf("versionMajor(%q)=%d want %d", in, got, want)
		}
	}
}

func TestPackagesForUbuntu2604(t *testing.T) {
	t.Parallel()
	pkgs, optional, notes := packagesFor(FamilyDebian, "ubuntu", "26.04")
	if len(pkgs) == 0 {
		t.Fatal("expected required packages")
	}
	need := []string{"tigervnc-standalone-server", "novnc", "websockify", "xfce4"}
	for _, n := range need {
		if !contains(pkgs, n) {
			t.Errorf("missing required package %q in %v", n, pkgs)
		}
	}
	if len(optional) == 0 {
		t.Fatal("expected optional packages")
	}
	joined := strings.Join(notes, " ")
	if !strings.Contains(joined, "26.04") {
		t.Fatalf("notes should mention 26.04: %v", notes)
	}
	if !strings.Contains(joined, "TigerVNC 1.15") {
		t.Fatalf("notes should mention TigerVNC 1.15 path for Ubuntu 24+/26: %v", notes)
	}
}

func TestPackagesForFedora(t *testing.T) {
	t.Parallel()
	pkgs, optional, notes := packagesFor(FamilyRHEL, "fedora", "44")
	need := []string{"xfwm4", "xfdesktop", "Thunar", "xfce4-panel", "xfce4-settings", "tigervnc-server"}
	for _, n := range need {
		if !slices.Contains(pkgs, n) {
			t.Errorf("missing required package %q in %v", n, pkgs)
		}
	}
	if len(optional) == 0 {
		t.Fatal("expected optional packages")
	}
	joined := strings.Join(notes, " ")
	if !strings.Contains(joined, "xfwm4") {
		t.Fatalf("notes should mention XFCE components: %v", notes)
	}
}

func TestPackagesForUnknown(t *testing.T) {
	t.Parallel()
	pkgs, _, notes := packagesFor(FamilyUnknown, "void", "1")
	if len(pkgs) != 0 {
		t.Fatalf("expected no packages, got %v", pkgs)
	}
	if len(notes) == 0 {
		t.Fatal("expected unsupported note")
	}
}

func TestBuildSetupScriptDebian(t *testing.T) {
	t.Parallel()
	plan := HostPlan{
		Supported:        true,
		Family:           FamilyDebian,
		PackageManager:   "apt-get",
		Distro:           "Ubuntu",
		DistroID:         "ubuntu",
		DistroVersion:    "26.04",
		Arch:             "amd64",
		DeviceType:       "host",
		Packages:         []string{"novnc", "xfce4", "tigervnc-standalone-server"},
		OptionalPackages: []string{"xfce4-goodies"},
	}
	script, err := BuildSetupScript(plan)
	if err != nil {
		t.Fatal(err)
	}
	mustContain := []string{
		"apt-get update",
		"novnc",
		"containerws-vnc-user-start",
		"containerws-novnc-user",
		"NOT enabled",
		"services_started\":false",
		"127.0.0.1:${NOVNC_PORT}",
		"-rfbport",
		"-localhost yes",
	}
	for _, s := range mustContain {
		if !strings.Contains(script, s) {
			t.Errorf("script missing %q", s)
		}
	}
	forbidden := []string{
		"systemctl enable",
		"systemctl restart",
		"WantedBy=multi-user.target",
		"0.0.0.0:${NOVNC_PORT}",
	}
	for _, s := range forbidden {
		if strings.Contains(script, s) {
			t.Errorf("setup script must not start/enable services; found %q", s)
		}
	}
}

func TestBuildSetupScriptUnsupported(t *testing.T) {
	t.Parallel()
	_, err := BuildSetupScript(HostPlan{
		Supported: false,
		OS:        "windows",
		Distro:    "Windows",
		DistroID:  "windows",
	})
	if err == nil {
		t.Fatal("expected error")
	}
	var ue *UnsupportedError
	if !errors.As(err, &ue) {
		t.Fatalf("want UnsupportedError, got %T: %v", err, err)
	}
}

func TestShellEscape(t *testing.T) {
	t.Parallel()
	in := `Ubuntu "26.04" $HOME ` + "`cmd`"
	out := shellEscape(in)
	if strings.Contains(out, "$") || strings.Contains(out, "`") {
		t.Fatalf("shellEscape left dangerous chars: %q", out)
	}
	if !strings.Contains(out, `\"`) {
		t.Fatalf("expected escaped quotes in %q", out)
	}
}

func TestDetectHostSmoke(t *testing.T) {
	plan := DetectHost()
	if plan.OS == "" {
		t.Fatal("expected OS from DetectHost")
	}
	if plan.Family == FamilyUnknown && plan.OS == "linux" {
		// Acceptable on exotic distros; still should not panic.
		t.Logf("unsupported family on this host: %+v", plan)
	}
	status := CheckStatus()
	if status.Plan.OS == "" {
		t.Fatal("status plan empty")
	}
	if status.Binaries == nil {
		t.Fatal("expected binaries map")
	}
}

func contains(list []string, want string) bool {
	return slices.Contains(list, want)
}
