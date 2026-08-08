package adduser

import (
	"bufio"
	"fmt"
	"math/rand"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	// BindHost is the default loopback listen address for panel-proxied noVNC.
	BindHost = "127.0.0.1"

	DefaultMapFile    = "/config/containerws/vnc-port-map.txt"
	DefaultSessionDir = "/config/containerws/vnc-sessions"

	// Legacy ranges kept for PickFreePort helpers / older map entries.
	DefaultVNCRangeStart   = 5901
	DefaultVNCRangeEnd     = 5999
	DefaultNoVNCRangeStart = 6080
	DefaultNoVNCRangeEnd   = 6179
)

var portMu sync.Mutex

// PortAssignment is username -> vnc + novnc ports (+ optional X display).
// Map line: username:vnc_port:novnc_port[:display]
type PortAssignment struct {
	Username  string `json:"username"`
	VncPort   int    `json:"vnc_port"`
	NoVncPort int    `json:"no_vnc_port"`
	Display   int    `json:"display,omitempty"`
}

func mapFilePath() string {
	if v := strings.TrimSpace(os.Getenv("CWS_VNC_PORT_MAP")); v != "" {
		return v
	}
	return DefaultMapFile
}

func sessionDir() string {
	if v := strings.TrimSpace(os.Getenv("CWS_VNC_SESSION_DIR")); v != "" {
		return v
	}
	return DefaultSessionDir
}

// LoadPortMap reads persisted assignments.
func LoadPortMap() ([]PortAssignment, error) {
	path := mapFilePath()
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()

	var out []PortAssignment
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.Split(line, ":")
		if len(parts) < 3 {
			continue
		}
		vnc, err1 := strconv.Atoi(parts[1])
		novnc, err2 := strconv.Atoi(parts[2])
		if err1 != nil || err2 != nil {
			continue
		}
		display := 0
		if len(parts) >= 4 {
			display, _ = strconv.Atoi(parts[3])
		}
		out = append(out, PortAssignment{
			Username:  parts[0],
			VncPort:   vnc,
			NoVncPort: novnc,
			Display:   display,
		})
	}
	return out, sc.Err()
}

func savePortMap(rows []PortAssignment) error {
	path := mapFilePath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	var b strings.Builder
	b.WriteString("# username:vnc_port:novnc_port[:display]  (bound to 127.0.0.1 only)\n")
	for _, row := range rows {
		if row.Display > 0 {
			fmt.Fprintf(&b, "%s:%d:%d:%d\n", row.Username, row.VncPort, row.NoVncPort, row.Display)
		} else {
			fmt.Fprintf(&b, "%s:%d:%d\n", row.Username, row.VncPort, row.NoVncPort)
		}
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(b.String()), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func usedPortsFromMap(rows []PortAssignment) map[int]struct{} {
	used := make(map[int]struct{}, len(rows)*2)
	for _, row := range rows {
		used[row.VncPort] = struct{}{}
		used[row.NoVncPort] = struct{}{}
	}
	return used
}

// PickUnusedLocalPort asks the OS for an unused TCP port on 127.0.0.1.
func PickUnusedLocalPort(used map[int]struct{}) (int, error) {
	for range 64 {
		ln, err := net.Listen("tcp", net.JoinHostPort(BindHost, "0"))
		if err != nil {
			return 0, fmt.Errorf("listen %s:0: %w", BindHost, err)
		}
		port := ln.Addr().(*net.TCPAddr).Port
		_ = ln.Close()
		if port < 1024 {
			continue
		}
		if _, ok := used[port]; ok {
			continue
		}
		// Confirm still free after close (race window is small).
		if !portIsFreeLocal(port) {
			continue
		}
		return port, nil
	}
	return 0, fmt.Errorf("no free localhost port available")
}

// PickFreePort finds a random free TCP port in [start,end] on 127.0.0.1.
func PickFreePort(start, end int, used map[int]struct{}) (int, error) {
	if end < start {
		return 0, fmt.Errorf("invalid port range %d-%d", start, end)
	}
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	span := end - start + 1
	for range 50 {
		candidate := start + rng.Intn(span)
		if _, ok := used[candidate]; ok {
			continue
		}
		if !portIsFreeLocal(candidate) {
			continue
		}
		return candidate, nil
	}
	for candidate := start; candidate <= end; candidate++ {
		if _, ok := used[candidate]; ok {
			continue
		}
		if portIsFreeLocal(candidate) {
			return candidate, nil
		}
	}
	return 0, fmt.Errorf("no free port found in range %d-%d", start, end)
}

func portIsFree(port int) bool {
	return portIsFreeLocal(port)
}

func portIsFreeLocal(port int) bool {
	ln, err := net.Listen("tcp", net.JoinHostPort(BindHost, strconv.Itoa(port)))
	if err != nil {
		return false
	}
	_ = ln.Close()
	return true
}

// IsPortListening reports whether something accepts TCP on host:port.
func IsPortListening(host string, port int) bool {
	if port <= 0 {
		return false
	}
	host = NormalizeBindAddress(host)
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(host, strconv.Itoa(port)), 400*time.Millisecond)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

// IsLocalPortListening reports whether something is accepting connections on BindHost:port.
func IsLocalPortListening(port int) bool {
	return IsPortListening(BindHost, port)
}

// SessionLogPath is the websockify log for a per-user session.
func SessionLogPath(username string) string {
	username = strings.TrimSpace(username)
	if username == "" {
		return ""
	}
	return filepath.Join(sessionDir(), username, "novnc.log")
}

// IsSessionLive is true when the session's noVNC (loopback) and/or VNC ports are listening.
func IsSessionLive(username string, vncPort, novncPort int) bool {
	return IsLocalPortListening(novncPort) || IsLocalPortListening(vncPort)
}

// IsSessionLiveOn reports live status using the configured VNC bind address.
func IsSessionLiveOn(vncBind string, vncPort, novncPort int) bool {
	if IsLocalPortListening(novncPort) {
		return true
	}
	return IsPortListening(vncBind, vncPort) || IsLocalPortListening(vncPort)
}

// PickFreeDisplay finds an unused X display number (:N).
func PickFreeDisplay() (int, error) {
	for d := 1; d <= 999; d++ {
		lock := fmt.Sprintf("/tmp/.X%d-lock", d)
		sock := fmt.Sprintf("/tmp/.X11-unix/X%d", d)
		if _, err := os.Stat(lock); err == nil {
			continue
		}
		if _, err := os.Stat(sock); err == nil {
			continue
		}
		return d, nil
	}
	return 0, fmt.Errorf("no free X display found")
}

// AllocateOrReusePorts returns existing map ports for username or allocates unused localhost ports.
func AllocateOrReusePorts(username string, extraUsed map[int]struct{}) (PortAssignment, error) {
	portMu.Lock()
	defer portMu.Unlock()

	rows, err := LoadPortMap()
	if err != nil {
		return PortAssignment{}, err
	}
	for _, row := range rows {
		if row.Username == username {
			// Reuse only if both ports are still free or already bound locally
			// (session may still be running). Always return stored assignment.
			return row, nil
		}
	}

	used := usedPortsFromMap(rows)
	for p := range extraUsed {
		used[p] = struct{}{}
	}

	vncPort, err := PickUnusedLocalPort(used)
	if err != nil {
		return PortAssignment{}, fmt.Errorf("allocate vnc port: %w", err)
	}
	used[vncPort] = struct{}{}
	novncPort, err := PickUnusedLocalPort(used)
	if err != nil {
		return PortAssignment{}, fmt.Errorf("allocate novnc port: %w", err)
	}

	row := PortAssignment{
		Username:  username,
		VncPort:   vncPort,
		NoVncPort: novncPort,
	}
	rows = append(rows, row)
	if err := savePortMap(rows); err != nil {
		return PortAssignment{}, err
	}
	return row, nil
}

// RemovePortAssignment drops a user from the map file.
func RemovePortAssignment(username string) error {
	portMu.Lock()
	defer portMu.Unlock()
	rows, err := LoadPortMap()
	if err != nil {
		return err
	}
	filtered := rows[:0]
	for _, row := range rows {
		if row.Username != username {
			filtered = append(filtered, row)
		}
	}
	return savePortMap(filtered)
}

// DisplayFromVNCPort maps classic TigerVNC TCP port → X display (5901 → :1).
// Prefer stored Display / PickFreeDisplay when using ephemeral OS ports.
func DisplayFromVNCPort(vncPort int) int {
	return vncPort - 5900
}

func saveDisplayFile(username string, display int) error {
	dir := filepath.Join(sessionDir(), username)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "display"), []byte(strconv.Itoa(display)+"\n"), 0o644)
}

func loadDisplayFile(username string) int {
	data, err := os.ReadFile(filepath.Join(sessionDir(), username, "display"))
	if err != nil {
		return 0
	}
	d, _ := strconv.Atoi(strings.TrimSpace(string(data)))
	return d
}
