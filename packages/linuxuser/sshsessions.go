package linuxuser

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// SSHConnection is an active login session for a Linux user (typically SSH).
type SSHConnection struct {
	ID           string `json:"id"`
	Username     string `json:"username"`
	TTY          string `json:"tty"`
	RemoteHost   string `json:"remote_host"`
	RemotePort   int    `json:"remote_port,omitempty"`
	LocalAddr    string `json:"local_addr,omitempty"`
	LocalPort    int    `json:"local_port,omitempty"`
	StartedAt    string `json:"started_at,omitempty"`
	Idle         string `json:"idle,omitempty"`
	PID          int    `json:"pid"`
	ShellPID     int    `json:"shell_pid,omitempty"`
	ShellCommand string `json:"shell_command,omitempty"`
	Command      string `json:"command,omitempty"`
	ViaSSH       bool   `json:"via_ssh"`
	Kind         string `json:"kind,omitempty"` // interactive | tunnel | local
}

// ListSSHConnections returns active sessions for username (SSH and local TTYs).
func ListSSHConnections(username string) ([]SSHConnection, error) {
	username = strings.TrimSpace(username)
	if username == "" {
		return nil, fmt.Errorf("username is required")
	}
	acc, err := Lookup(username)
	if err != nil {
		return nil, err
	}
	if acc == nil || !acc.Exists {
		return nil, fmt.Errorf("linux user %q does not exist", username)
	}

	byKey := map[string]SSHConnection{}

	for _, c := range sessionsFromWho(username) {
		byKey[sessionKey(c)] = c
	}
	for _, c := range sessionsFromSSHD(username) {
		k := sessionKey(c)
		if prev, ok := byKey[k]; ok {
			byKey[k] = mergeConnection(prev, c)
			continue
		}
		// Prefer tty match to enrich an existing who row.
		merged := false
		for pk, prev := range byKey {
			if prev.TTY != "" && prev.TTY == c.TTY {
				byKey[pk] = mergeConnection(prev, c)
				merged = true
				break
			}
		}
		if !merged {
			byKey[k] = c
		}
	}

	out := make([]SSHConnection, 0, len(byKey))
	for _, c := range byKey {
		enrichConnection(&c)
		if c.ID == "" {
			c.ID = connectionID(c)
		}
		out = append(out, c)
	}
	sortConnections(out)
	return out, nil
}

func enrichConnection(c *SSHConnection) {
	if c == nil || c.PID <= 0 {
		return
	}
	if info := sshInfoFromProcessTree(c.PID); info.remoteHost != "" {
		if c.RemoteHost == "" {
			c.RemoteHost = info.remoteHost
		}
		if c.RemotePort == 0 {
			c.RemotePort = info.remotePort
		}
		if c.LocalAddr == "" {
			c.LocalAddr = info.localAddr
		}
		if c.LocalPort == 0 {
			c.LocalPort = info.localPort
		}
		c.ViaSSH = true
	}
	if c.ShellPID == 0 || c.ShellCommand == "" {
		if spid, scmd := sessionShell(c.PID); spid > 0 {
			c.ShellPID = spid
			c.ShellCommand = scmd
			if info := sshInfoFromEnviron(spid); info.remoteHost != "" {
				c.RemoteHost = info.remoteHost
				if c.RemotePort == 0 {
					c.RemotePort = info.remotePort
				}
				if c.LocalAddr == "" {
					c.LocalAddr = info.localAddr
				}
				if c.LocalPort == 0 {
					c.LocalPort = info.localPort
				}
				c.ViaSSH = true
				if c.TTY == "" && info.tty != "" {
					c.TTY = info.tty
				}
			}
		}
	}
	if c.Kind == "" {
		switch {
		case c.ViaSSH && c.TTY != "":
			c.Kind = "interactive"
		case c.ViaSSH:
			c.Kind = "tunnel"
		default:
			c.Kind = "local"
		}
	}
}

// KillSSHConnection terminates a session belonging to username.
// id is the connection id from ListSSHConnections (pid-<n> or tty-…).
func KillSSHConnection(username, id string) error {
	username = strings.TrimSpace(username)
	id = strings.TrimSpace(id)
	if username == "" || id == "" {
		return fmt.Errorf("username and session id are required")
	}

	sessions, err := ListSSHConnections(username)
	if err != nil {
		return err
	}
	var target *SSHConnection
	for i := range sessions {
		if sessions[i].ID == id {
			target = &sessions[i]
			break
		}
	}
	if target == nil {
		return fmt.Errorf("session %q not found for user %s", id, username)
	}
	if target.PID <= 1 {
		return fmt.Errorf("session has no killable process")
	}

	// Prefer terminating the privileged parent so the whole SSH tree exits.
	killPID := target.PID
	if ppid := parentPID(target.PID); ppid > 1 {
		pcmd := processCmdline(ppid)
		if strings.Contains(pcmd, "[priv]") &&
			(strings.Contains(pcmd, "sshd-session: "+username) || strings.Contains(pcmd, "sshd: "+username)) {
			killPID = ppid
		}
	}

	if !processBelongsToUserSession(target.PID, username, target.TTY) &&
		!processBelongsToUserSession(killPID, username, target.TTY) {
		return fmt.Errorf("refusing to kill pid %d — ownership check failed", target.PID)
	}

	proc, err := os.FindProcess(killPID)
	if err != nil {
		return fmt.Errorf("find process: %w", err)
	}
	if err := proc.Signal(syscall.SIGTERM); err != nil {
		// Fallback: pkill by tty when allowed.
		if tty := normalizeTTY(target.TTY); tty != "" {
			_ = exec.Command("pkill", "-TERM", "-t", tty, "-u", username).Run()
		}
		return fmt.Errorf("signal pid %d: %w", killPID, err)
	}

	// Give the session a moment, then escalate if still alive.
	time.Sleep(200 * time.Millisecond)
	if processAlive(killPID) {
		_ = proc.Signal(syscall.SIGKILL)
	}
	return nil
}

func sessionsFromWho(username string) []SSHConnection {
	out, err := exec.Command("who", "-u").Output()
	if err != nil {
		out, err = exec.Command("who").Output()
		if err != nil {
			return nil
		}
	}

	var sessions []SSHConnection
	sc := bufio.NewScanner(strings.NewReader(string(out)))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		// Skip header lines from who -uH
		if strings.HasPrefix(strings.ToUpper(line), "NAME") && strings.Contains(line, "LINE") {
			continue
		}
		c, ok := parseWhoLine(line, username)
		if !ok {
			continue
		}
		sessions = append(sessions, c)
	}
	return sessions
}

// parseWhoLine handles common util-linux who -u layouts, e.g.
//
//	root pts/0 2026-07-27 12:00 . 12345 (192.168.1.5)
//	root pts/0 Jul 27 12:00 00:01 12345 (host.example)
func parseWhoLine(line, username string) (SSHConnection, bool) {
	fields := strings.Fields(line)
	if len(fields) < 2 {
		return SSHConnection{}, false
	}
	if fields[0] != username {
		return SSHConnection{}, false
	}
	c := SSHConnection{
		Username: username,
		TTY:      normalizeTTY(fields[1]),
	}

	// Remote host is usually the last "(…)" token.
	if i := strings.LastIndex(line, "("); i >= 0 && strings.HasSuffix(line, ")") {
		c.RemoteHost = strings.TrimSpace(line[i+1 : len(line)-1])
		c.ViaSSH = c.RemoteHost != "" && c.RemoteHost != ":0" && c.RemoteHost != ":0.0"
	}

	// PID: last integer field that isn't part of a date.
	for i := len(fields) - 1; i >= 2; i-- {
		f := fields[i]
		if strings.HasPrefix(f, "(") {
			continue
		}
		if pid, err := strconv.Atoi(f); err == nil && pid > 1 {
			c.PID = pid
			break
		}
	}

	// Idle: often a field like "." / "old" / "00:01" before the PID.
	if c.PID > 0 {
		for i, f := range fields {
			if f == strconv.Itoa(c.PID) && i > 2 {
				prev := fields[i-1]
				if prev == "." || prev == "old" || strings.Contains(prev, ":") || prev == "idle" {
					c.Idle = prev
				}
				break
			}
		}
	}

	// Started-at: best-effort — take middle tokens that look like date/time.
	if len(fields) >= 4 {
		// Prefer ISO-ish "2026-07-27 12:00"
		if looksDate(fields[2]) {
			c.StartedAt = fields[2]
			if len(fields) > 3 && looksTime(fields[3]) {
				c.StartedAt += " " + fields[3]
			}
		} else if len(fields) >= 5 {
			// "Jul 27 12:00"
			c.StartedAt = strings.Join(fields[2:5], " ")
		}
	}

	if c.PID > 0 {
		if remote := remoteHostFromProc(c.PID); remote != "" {
			c.RemoteHost = remote
			c.ViaSSH = true
		}
	}
	c.ID = connectionID(c)
	c.Command = processCmdline(c.PID)
	return c, true
}

func sessionsFromSSHD(username string) []SSHConnection {
	out, err := exec.Command("ps", "-eo", "pid=,user=,tty=,lstart=,args=").Output()
	if err != nil {
		return nil
	}
	remotes := endpointsFromSS()

	var sessions []SSHConnection
	sc := bufio.NewScanner(strings.NewReader(string(out)))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 8 {
			continue
		}
		pid, err := strconv.Atoi(fields[0])
		if err != nil || pid <= 1 {
			continue
		}

		args := strings.Join(fields[8:], " ")
		sessionUser, ttyFromArgs, ok := parseSSHDSessionArgs(args)
		if !ok || sessionUser != username {
			continue
		}

		tty := normalizeTTY(fields[2])
		if tty == "" {
			tty = normalizeTTY(ttyFromArgs)
		}
		started := strings.Join(fields[3:8], " ")

		ep := remotes[pid]
		if ep.remoteHost == "" {
			if ppid := parentPID(pid); ppid > 1 {
				ep = remotes[ppid]
			}
		}
		if ep.remoteHost == "" {
			ep = sshInfoFromEnviron(pid)
		}

		c := SSHConnection{
			Username:   username,
			TTY:        tty,
			PID:        pid,
			StartedAt:  started,
			Command:    args,
			ViaSSH:     true,
			RemoteHost: ep.remoteHost,
			RemotePort: ep.remotePort,
			LocalAddr:  ep.localAddr,
			LocalPort:  ep.localPort,
		}
		if tty != "" {
			c.Kind = "interactive"
		} else {
			c.Kind = "tunnel"
		}
		c.ID = connectionID(c)
		sessions = append(sessions, c)
	}
	return sessions
}

// parseSSHDSessionArgs extracts user + tty from sshd / sshd-session cmdline.
// Skips listener and privileged helper processes.
func parseSSHDSessionArgs(args string) (user, tty string, ok bool) {
	args = strings.TrimSpace(args)
	var rest string
	switch {
	case strings.HasPrefix(args, "sshd-session:"):
		rest = strings.TrimSpace(strings.TrimPrefix(args, "sshd-session:"))
	case strings.HasPrefix(args, "sshd:"):
		rest = strings.TrimSpace(strings.TrimPrefix(args, "sshd:"))
	default:
		return "", "", false
	}
	// Listener: "/usr/sbin/sshd [listener]…"
	if strings.HasPrefix(rest, "/") || strings.Contains(rest, "[listener]") {
		return "", "", false
	}
	// Privileged helper: "root [priv]" — not a user session row.
	if strings.Contains(rest, "[priv]") {
		return "", "", false
	}
	// Expected: "username@pts/0" or "username@notty"
	at := strings.IndexByte(rest, '@')
	if at <= 0 {
		return "", "", false
	}
	user = strings.TrimSpace(rest[:at])
	ttyPart := strings.Fields(rest[at+1:])
	if len(ttyPart) == 0 || user == "" {
		return "", "", false
	}
	tty = strings.TrimSuffix(ttyPart[0], ")")
	if tty == "notty" {
		tty = ""
	}
	return user, tty, true
}

// remoteHostsFromSS maps sshd/sshd-session PIDs to peer IPs for established :22 sockets.
type ssEndpoint struct {
	remoteHost string
	remotePort int
	localAddr  string
	localPort  int
	tty        string
}

func endpointsFromSS() map[int]ssEndpoint {
	out := map[int]ssEndpoint{}
	raw, err := exec.Command("ss", "-tnp").Output()
	if err != nil {
		return out
	}
	sc := bufio.NewScanner(strings.NewReader(string(raw)))
	for sc.Scan() {
		line := sc.Text()
		if !strings.Contains(line, "users:(") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 5 {
			continue
		}
		local := fields[3]
		peer := fields[4]
		localHost, localPort := splitHostPort(local)
		remoteHost, remotePort := splitHostPort(peer)
		if remoteHost == "" {
			continue
		}
		// Prefer the inbound SSH listener socket (local port 22).
		if localPort != 22 && localPort != 0 {
			continue
		}
		ep := ssEndpoint{
			remoteHost: remoteHost,
			remotePort: remotePort,
			localAddr:  localHost,
			localPort:  localPort,
		}
		for _, pid := range pidsFromSSUsers(line) {
			if _, exists := out[pid]; !exists {
				out[pid] = ep
			}
		}
	}
	return out
}

func splitHostPort(addr string) (host string, port int) {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return "", 0
	}
	if strings.HasPrefix(addr, "[") {
		end := strings.IndexByte(addr, ']')
		if end > 1 {
			host = addr[1:end]
			rest := addr[end+1:]
			if strings.HasPrefix(rest, ":") {
				port, _ = strconv.Atoi(rest[1:])
			}
			return host, port
		}
	}
	i := strings.LastIndexByte(addr, ':')
	if i < 0 {
		return addr, 0
	}
	host = addr[:i]
	port, _ = strconv.Atoi(addr[i+1:])
	return host, port
}

func peerHost(peer string) string {
	host, _ := splitHostPort(peer)
	return host
}

func pidsFromSSUsers(line string) []int {
	// users:(("sshd-session",pid=129119,fd=7),("sshd-session",pid=129107,fd=7))
	var pids []int
	rest := line
	for {
		i := strings.Index(rest, "pid=")
		if i < 0 {
			break
		}
		rest = rest[i+4:]
		n := 0
		for n < len(rest) && rest[n] >= '0' && rest[n] <= '9' {
			n++
		}
		if n == 0 {
			continue
		}
		pid, err := strconv.Atoi(rest[:n])
		if err == nil && pid > 1 {
			pids = append(pids, pid)
		}
		rest = rest[n:]
	}
	return pids
}

func sshInfoFromProcessTree(pid int) ssEndpoint {
	if ep := sshInfoFromEnviron(pid); ep.remoteHost != "" {
		return ep
	}
	// Walk a few parents / children for SSH_* env.
	if ppid := parentPID(pid); ppid > 1 {
		if ep := sshInfoFromEnviron(ppid); ep.remoteHost != "" {
			return ep
		}
	}
	for _, child := range childPIDs(pid) {
		if ep := sshInfoFromEnviron(child); ep.remoteHost != "" {
			return ep
		}
	}
	return ssEndpoint{}
}

func sshInfoFromEnviron(pid int) ssEndpoint {
	if pid <= 1 {
		return ssEndpoint{}
	}
	raw, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "environ"))
	if err != nil {
		return ssEndpoint{}
	}
	ep := ssEndpoint{}
	for entry := range strings.SplitSeq(string(raw), "\x00") {
		switch {
		case strings.HasPrefix(entry, "SSH_CONNECTION="):
			parts := strings.Fields(strings.TrimPrefix(entry, "SSH_CONNECTION="))
			// client_ip client_port server_ip server_port
			if len(parts) >= 1 {
				ep.remoteHost = parts[0]
			}
			if len(parts) >= 2 {
				ep.remotePort, _ = strconv.Atoi(parts[1])
			}
			if len(parts) >= 3 {
				ep.localAddr = parts[2]
			}
			if len(parts) >= 4 {
				ep.localPort, _ = strconv.Atoi(parts[3])
			}
		case strings.HasPrefix(entry, "SSH_CLIENT=") && ep.remoteHost == "":
			parts := strings.Fields(strings.TrimPrefix(entry, "SSH_CLIENT="))
			if len(parts) >= 1 {
				ep.remoteHost = parts[0]
			}
			if len(parts) >= 2 {
				ep.remotePort, _ = strconv.Atoi(parts[1])
			}
			if len(parts) >= 3 {
				ep.localPort, _ = strconv.Atoi(parts[2])
			}
		case strings.HasPrefix(entry, "SSH_TTY="):
			ep.tty = normalizeTTY(strings.TrimPrefix(entry, "SSH_TTY="))
		}
	}
	return ep
}

func sessionShell(sshdPID int) (pid int, cmd string) {
	children := childPIDs(sshdPID)
	for _, cpid := range children {
		cmdline := strings.TrimSpace(processCmdline(cpid))
		if cmdline == "" {
			continue
		}
		fields := strings.Fields(cmdline)
		base := filepath.Base(fields[0])
		base = strings.TrimPrefix(base, "-")
		switch base {
		case "bash", "sh", "zsh", "fish", "dash", "ksh", "tmux", "screen":
			return cpid, cmdline
		}
	}
	if len(children) > 0 {
		return children[0], processCmdline(children[0])
	}
	return 0, ""
}

func childPIDs(pid int) []int {
	entries, err := os.ReadDir(filepath.Join("/proc", strconv.Itoa(pid), "task"))
	if err != nil {
		// Fallback: scan /proc for PPid match (slower).
		return childPIDsScan(pid)
	}
	seen := map[int]struct{}{}
	var out []int
	for _, ent := range entries {
		raw, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "task", ent.Name(), "children"))
		if err != nil {
			continue
		}
		for f := range strings.FieldsSeq(string(raw)) {
			cpid, err := strconv.Atoi(f)
			if err != nil || cpid <= 1 {
				continue
			}
			if _, ok := seen[cpid]; ok {
				continue
			}
			seen[cpid] = struct{}{}
			out = append(out, cpid)
		}
	}
	if len(out) == 0 {
		return childPIDsScan(pid)
	}
	return out
}

func childPIDsScan(pid int) []int {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil
	}
	var out []int
	for _, ent := range entries {
		if !ent.IsDir() {
			continue
		}
		cpid, err := strconv.Atoi(ent.Name())
		if err != nil || cpid <= 1 {
			continue
		}
		if parentPID(cpid) == pid {
			out = append(out, cpid)
		}
	}
	return out
}

func mergeConnection(a, b SSHConnection) SSHConnection {
	out := a
	if out.PID <= 0 {
		out.PID = b.PID
	}
	if out.TTY == "" {
		out.TTY = b.TTY
	}
	if out.RemoteHost == "" {
		out.RemoteHost = b.RemoteHost
	}
	if out.RemotePort == 0 {
		out.RemotePort = b.RemotePort
	}
	if out.LocalAddr == "" {
		out.LocalAddr = b.LocalAddr
	}
	if out.LocalPort == 0 {
		out.LocalPort = b.LocalPort
	}
	if out.StartedAt == "" {
		out.StartedAt = b.StartedAt
	}
	if out.Idle == "" {
		out.Idle = b.Idle
	}
	if out.Command == "" {
		out.Command = b.Command
	}
	if out.ShellPID == 0 {
		out.ShellPID = b.ShellPID
	}
	if out.ShellCommand == "" {
		out.ShellCommand = b.ShellCommand
	}
	if out.Kind == "" {
		out.Kind = b.Kind
	}
	out.ViaSSH = out.ViaSSH || b.ViaSSH
	out.ID = connectionID(out)
	return out
}

func sessionKey(c SSHConnection) string {
	if c.TTY != "" {
		return "tty:" + c.TTY
	}
	if c.PID > 0 {
		return "pid:" + strconv.Itoa(c.PID)
	}
	return c.ID
}

func connectionID(c SSHConnection) string {
	if c.PID > 0 {
		return "pid-" + strconv.Itoa(c.PID)
	}
	if c.TTY != "" {
		return "tty-" + strings.ReplaceAll(c.TTY, "/", "-")
	}
	return "unknown"
}

func normalizeTTY(tty string) string {
	tty = strings.TrimSpace(tty)
	if tty == "" || tty == "?" || tty == "-" {
		return ""
	}
	return strings.TrimPrefix(tty, "/dev/")
}

func remoteHostFromProc(pid int) string {
	if pid <= 1 {
		return ""
	}
	envPath := filepath.Join("/proc", strconv.Itoa(pid), "environ")
	raw, err := os.ReadFile(envPath)
	if err != nil {
		// Try parent process (sshd session → shell).
		ppid := parentPID(pid)
		if ppid > 1 {
			raw, err = os.ReadFile(filepath.Join("/proc", strconv.Itoa(ppid), "environ"))
		}
		if err != nil {
			return ""
		}
	}
	for entry := range strings.SplitSeq(string(raw), "\x00") {
		if v, ok := strings.CutPrefix(entry, "SSH_CLIENT="); ok {
			parts := strings.Fields(v)
			if len(parts) > 0 {
				return parts[0]
			}
		}
		if v, ok := strings.CutPrefix(entry, "SSH_CONNECTION="); ok {
			parts := strings.Fields(v)
			if len(parts) > 0 {
				return parts[0]
			}
		}
	}
	return ""
}

func parentPID(pid int) int {
	raw, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "stat"))
	if err != nil {
		return 0
	}
	// /proc/pid/stat: pid (comm) state ppid ...
	s := string(raw)
	i := strings.LastIndex(s, ")")
	if i < 0 || i+2 >= len(s) {
		return 0
	}
	fields := strings.Fields(s[i+2:])
	if len(fields) < 2 {
		return 0
	}
	ppid, _ := strconv.Atoi(fields[1])
	return ppid
}

func processCmdline(pid int) string {
	if pid <= 1 {
		return ""
	}
	raw, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "cmdline"))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(strings.ReplaceAll(string(raw), "\x00", " "))
}

func processBelongsToUserSession(pid int, username, tty string) bool {
	cmdline := processCmdline(pid)
	if user, _, ok := parseSSHDSessionArgs(cmdline); ok && user == username {
		return true
	}
	// Also allow privileged helper for the same user (used when disconnecting).
	if strings.Contains(cmdline, "sshd-session: "+username+" ") ||
		strings.Contains(cmdline, "sshd: "+username+" ") ||
		strings.Contains(cmdline, "sshd-session: "+username+"@") ||
		strings.Contains(cmdline, "sshd: "+username+"@") {
		return true
	}
	// Login shell / process owned by the user.
	raw, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "status"))
	if err != nil {
		return false
	}
	uid := ""
	for line := range strings.SplitSeq(string(raw), "\n") {
		if v, ok := strings.CutPrefix(line, "Uid:"); ok {
			fields := strings.Fields(v)
			if len(fields) > 0 {
				uid = fields[0]
			}
			break
		}
	}
	acc, err := Lookup(username)
	if err != nil || acc == nil || !acc.Exists {
		return false
	}
	if uid != "" && uid == acc.UID {
		return true
	}
	// Last resort: tty match via ps.
	if tty != "" {
		out, err := exec.Command("ps", "-p", strconv.Itoa(pid), "-o", "tty=").Output()
		if err == nil && normalizeTTY(string(out)) == normalizeTTY(tty) {
			return true
		}
	}
	return false
}

func processAlive(pid int) bool {
	_, err := os.Stat(filepath.Join("/proc", strconv.Itoa(pid)))
	return err == nil
}

func looksDate(s string) bool {
	return strings.Contains(s, "-") && len(s) >= 8
}

func looksTime(s string) bool {
	return strings.Count(s, ":") >= 1 && len(s) <= 8
}

func sortConnections(items []SSHConnection) {
	for i := range items {
		for j := i + 1; j < len(items); j++ {
			if items[j].TTY < items[i].TTY ||
				(items[j].TTY == items[i].TTY && items[j].PID < items[i].PID) {
				items[i], items[j] = items[j], items[i]
			}
		}
	}
}
