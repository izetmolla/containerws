package adduser

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDisplayFromVNCPort(t *testing.T) {
	t.Parallel()
	if got := DisplayFromVNCPort(5901); got != 1 {
		t.Fatalf("5901 → %d want 1", got)
	}
	if got := DisplayFromVNCPort(5910); got != 10 {
		t.Fatalf("5910 → %d want 10", got)
	}
}

func TestPickUnusedLocalPort(t *testing.T) {
	t.Parallel()
	used := map[int]struct{}{}
	p1, err := PickUnusedLocalPort(used)
	if err != nil {
		t.Fatal(err)
	}
	used[p1] = struct{}{}
	p2, err := PickUnusedLocalPort(used)
	if err != nil {
		t.Fatal(err)
	}
	if p1 == p2 {
		t.Fatalf("expected distinct ports, got %d twice", p1)
	}
	if p1 < 1024 || p2 < 1024 {
		t.Fatalf("privileged ports: %d %d", p1, p2)
	}
}

func TestPickFreeDisplay(t *testing.T) {
	d, err := PickFreeDisplay()
	if err != nil {
		t.Fatal(err)
	}
	if d < 1 {
		t.Fatalf("display %d", d)
	}
}

func TestPickFreePortSkipsUsedAndListening(t *testing.T) {
	t.Parallel()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	listening := ln.Addr().(*net.TCPAddr).Port

	start, end := listening, listening
	used := map[int]struct{}{}
	if _, err := PickFreePort(start, end, used); err == nil {
		t.Fatal("expected error when only candidate is listening")
	}

	used[listening] = struct{}{}
	// Use a tiny free range that excludes the listening port by marking it used,
	// and pick within ephemeral-friendly high ports that should be free.
	freeStart, freeEnd := 45000, 45020
	port, err := PickFreePort(freeStart, freeEnd, used)
	if err != nil {
		t.Fatal(err)
	}
	if port < freeStart || port > freeEnd {
		t.Fatalf("port %d outside range", port)
	}
	if _, ok := used[port]; ok {
		t.Fatalf("picked used port %d", port)
	}
	if !portIsFreeLocal(port) {
		t.Fatalf("picked busy port %d", port)
	}
}

func TestPickFreePortInvalidRange(t *testing.T) {
	t.Parallel()
	if _, err := PickFreePort(10, 5, nil); err == nil {
		t.Fatal("expected invalid range error")
	}
}

func TestPortMapAllocateReuseRemove(t *testing.T) {
	dir := t.TempDir()
	mapPath := filepath.Join(dir, "port-map.txt")
	t.Setenv("CWS_VNC_PORT_MAP", mapPath)

	rows, err := LoadPortMap()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 0 {
		t.Fatalf("expected empty map, got %v", rows)
	}

	a1, err := AllocateOrReusePorts("alice", nil)
	if err != nil {
		t.Fatal(err)
	}
	if a1.Username != "alice" {
		t.Fatalf("username %q", a1.Username)
	}
	if a1.VncPort < 1024 || a1.NoVncPort < 1024 {
		t.Fatalf("ports should be unprivileged: %+v", a1)
	}
	if a1.VncPort == a1.NoVncPort {
		t.Fatal("vnc and novnc ports must differ")
	}
	if !portIsFreeLocal(a1.VncPort) || !portIsFreeLocal(a1.NoVncPort) {
		t.Fatalf("allocated ports should be free on 127.0.0.1: %+v", a1)
	}

	a2, err := AllocateOrReusePorts("alice", nil)
	if err != nil {
		t.Fatal(err)
	}
	if a2.VncPort != a1.VncPort || a2.NoVncPort != a1.NoVncPort {
		t.Fatalf("reuse mismatch: %+v vs %+v", a1, a2)
	}

	b1, err := AllocateOrReusePorts("bob", map[int]struct{}{a1.VncPort: {}, a1.NoVncPort: {}})
	if err != nil {
		t.Fatal(err)
	}
	if b1.VncPort == a1.VncPort || b1.NoVncPort == a1.NoVncPort {
		t.Fatalf("bob collided with alice: %+v / %+v", a1, b1)
	}

	all, err := LoadPortMap()
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 2 {
		t.Fatalf("want 2 rows, got %d: %v", len(all), all)
	}

	if err := RemovePortAssignment("alice"); err != nil {
		t.Fatal(err)
	}
	all, err = LoadPortMap()
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 1 || all[0].Username != "bob" {
		t.Fatalf("after remove alice: %v", all)
	}

	raw, err := os.ReadFile(mapPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), fmt.Sprintf("bob:%d:%d", b1.VncPort, b1.NoVncPort)) {
		t.Fatalf("map file missing bob line:\n%s", raw)
	}
}

func TestLoadPortMapSkipsBadLines(t *testing.T) {
	dir := t.TempDir()
	mapPath := filepath.Join(dir, "port-map.txt")
	t.Setenv("CWS_VNC_PORT_MAP", mapPath)

	content := `# comment
alice:5901:6080

badline
carol:notaport:6081
dave:5902:6082
`
	if err := os.WriteFile(mapPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	rows, err := LoadPortMap()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("want 2 valid rows, got %v", rows)
	}
	if rows[0].Username != "alice" || rows[0].VncPort != 5901 || rows[0].NoVncPort != 6080 {
		t.Fatalf("alice row: %+v", rows[0])
	}
	if rows[1].Username != "dave" {
		t.Fatalf("dave row: %+v", rows[1])
	}
}

func TestMapFilePathEnv(t *testing.T) {
	t.Setenv("CWS_VNC_PORT_MAP", "/tmp/custom-map.txt")
	if got := mapFilePath(); got != "/tmp/custom-map.txt" {
		t.Fatalf("got %q", got)
	}
	t.Setenv("CWS_VNC_PORT_MAP", "")
	if got := mapFilePath(); got != DefaultMapFile {
		t.Fatalf("default got %q", got)
	}
}

func TestUpsertAssignment(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("CWS_VNC_PORT_MAP", filepath.Join(dir, "map.txt"))

	if err := upsertAssignment(PortAssignment{Username: "eve", VncPort: 5911, NoVncPort: 6091}); err != nil {
		t.Fatal(err)
	}
	if err := upsertAssignment(PortAssignment{Username: "eve", VncPort: 5912, NoVncPort: 6092}); err != nil {
		t.Fatal(err)
	}
	rows, err := LoadPortMap()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("want 1 row after upsert, got %v", rows)
	}
	if rows[0].VncPort != 5912 || rows[0].NoVncPort != 6092 {
		t.Fatalf("upsert not applied: %+v", rows[0])
	}
}
