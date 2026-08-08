package machine

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

// ProcessInfo is a live process snapshot for the dashboard.
type ProcessInfo struct {
	PID           int     `json:"pid"`
	PPID          int     `json:"ppid"`
	Name          string  `json:"name"`
	User          string  `json:"user"`
	Cmdline       string  `json:"cmdline"`
	State         string  `json:"state"`
	Threads       int     `json:"threads"`
	CPUPercent    float64 `json:"cpu_percent"`
	MemoryBytes   int64   `json:"memory_bytes"`
	MemoryHuman   string  `json:"memory_human"`
	MemoryPercent float64 `json:"memory_percent"`
}

type procCPUSample struct {
	totalTicks uint64
	at         time.Time
}

type processPrevState struct {
	mu      sync.Mutex
	samples map[int]procCPUSample
}

var processPrev processPrevState

const linuxClockTicks = 100.0

// CollectProcesses scans /proc and returns processes sorted by CPU then memory.
// CPU% uses deltas from the previous sample when available.
func CollectProcesses(limit int) []ProcessInfo {
	if limit <= 0 {
		limit = 50
	}

	pageSize := int64(os.Getpagesize())
	if pageSize <= 0 {
		pageSize = 4096
	}

	memTotal := readMemTotalBytes()
	now := time.Now()
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil
	}

	type rawProc struct {
		info       ProcessInfo
		totalTicks uint64
	}

	raw := make([]rawProc, 0, 128)
	for _, ent := range entries {
		if !ent.IsDir() {
			continue
		}
		pid, err := strconv.Atoi(ent.Name())
		if err != nil || pid <= 0 {
			continue
		}
		info, ticks, ok := readProcess(pid, pageSize, memTotal)
		if !ok {
			continue
		}
		raw = append(raw, rawProc{info: info, totalTicks: ticks})
	}

	processPrev.mu.Lock()

	if processPrev.samples == nil {
		processPrev.samples = make(map[int]procCPUSample, len(raw))
	}

	// First call: seed samples and take a short second pass for CPU%.
	if len(processPrev.samples) == 0 {
		for _, p := range raw {
			processPrev.samples[p.info.PID] = procCPUSample{totalTicks: p.totalTicks, at: now}
		}
		processPrev.mu.Unlock()
		time.Sleep(120 * time.Millisecond)
		return CollectProcesses(limit)
	}

	next := make(map[int]procCPUSample, len(raw))
	out := make([]ProcessInfo, 0, len(raw))
	for _, p := range raw {
		info := p.info
		prev, ok := processPrev.samples[p.info.PID]
		if ok {
			elapsed := now.Sub(prev.at).Seconds()
			if elapsed > 0.05 {
				delta := float64(p.totalTicks - prev.totalTicks)
				if p.totalTicks >= prev.totalTicks && delta >= 0 {
					info.CPUPercent = round2((delta / linuxClockTicks / elapsed) * 100)
					if info.CPUPercent < 0 {
						info.CPUPercent = 0
					}
				}
			}
		}
		next[p.info.PID] = procCPUSample{totalTicks: p.totalTicks, at: now}
		out = append(out, info)
	}
	processPrev.samples = next
	processPrev.mu.Unlock()

	sort.SliceStable(out, func(i, j int) bool {
		if out[i].CPUPercent != out[j].CPUPercent {
			return out[i].CPUPercent > out[j].CPUPercent
		}
		if out[i].MemoryBytes != out[j].MemoryBytes {
			return out[i].MemoryBytes > out[j].MemoryBytes
		}
		return out[i].PID < out[j].PID
	})

	if len(out) > limit {
		out = out[:limit]
	}
	return out
}

// KillProcess sends SIGTERM (or SIGKILL when force) to pid.
// Refuses pid 1 and the current process.
func KillProcess(pid int, force bool) error {
	if pid <= 1 {
		return fmt.Errorf("refusing to kill system process %d", pid)
	}
	if pid == os.Getpid() {
		return fmt.Errorf("refusing to kill the current process")
	}

	proc, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	// Ensure the process still exists.
	if err := proc.Signal(syscall.Signal(0)); err != nil {
		return fmt.Errorf("process %d is not running", pid)
	}

	sig := syscall.SIGTERM
	if force {
		sig = syscall.SIGKILL
	}
	if err := proc.Signal(sig); err != nil {
		return err
	}

	if force {
		return nil
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if err := proc.Signal(syscall.Signal(0)); err != nil {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	_ = proc.Signal(syscall.SIGKILL)
	return nil
}

func readProcess(pid int, pageSize, memTotal int64) (ProcessInfo, uint64, bool) {
	base := filepath.Join("/proc", strconv.Itoa(pid))
	statBytes, err := os.ReadFile(filepath.Join(base, "stat"))
	if err != nil {
		return ProcessInfo{}, 0, false
	}
	statLine := string(statBytes)
	// comm is wrapped in parentheses and may contain spaces.
	lparen := strings.IndexByte(statLine, '(')
	rparen := strings.LastIndexByte(statLine, ')')
	if lparen < 0 || rparen < 0 || rparen <= lparen {
		return ProcessInfo{}, 0, false
	}
	name := statLine[lparen+1 : rparen]
	fields := strings.Fields(statLine[rparen+1:])
	// After ")": state(0), ppid(1), ... utime(11), stime(12), ... rss(21), ... num_threads(17)
	if len(fields) < 22 {
		return ProcessInfo{}, 0, false
	}
	state := fields[0]
	ppid, _ := strconv.Atoi(fields[1])
	utime, _ := strconv.ParseUint(fields[11], 10, 64)
	stime, _ := strconv.ParseUint(fields[12], 10, 64)
	threads, _ := strconv.Atoi(fields[17])
	rssPages, _ := strconv.ParseInt(fields[21], 10, 64)
	if rssPages < 0 {
		rssPages = 0
	}
	memBytes := rssPages * pageSize

	cmdline := readCmdline(filepath.Join(base, "cmdline"))
	if cmdline == "" {
		cmdline = name
	}
	uid := readStatusUID(filepath.Join(base, "status"))

	info := ProcessInfo{
		PID:         pid,
		PPID:        ppid,
		Name:        name,
		User:        resolveUser(uid),
		Cmdline:     truncate(cmdline, 180),
		State:       state,
		Threads:     threads,
		MemoryBytes: memBytes,
		MemoryHuman: humanBytes(memBytes),
	}
	if info.MemoryHuman == "" {
		info.MemoryHuman = "0 B"
	}
	if memTotal > 0 {
		info.MemoryPercent = round2(float64(memBytes) / float64(memTotal) * 100)
	}
	return info, utime + stime, true
}

func readCmdline(path string) string {
	b, err := os.ReadFile(path)
	if err != nil || len(b) == 0 {
		return ""
	}
	parts := strings.Split(string(b), "\x00")
	cleaned := make([]string, 0, len(parts))
	for _, p := range parts {
		if p == "" {
			continue
		}
		cleaned = append(cleaned, p)
	}
	return strings.Join(cleaned, " ")
}

func readStatusUID(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	for line := range strings.SplitSeq(string(b), "\n") {
		if !strings.HasPrefix(line, "Uid:") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) >= 2 {
			return fields[1]
		}
	}
	return ""
}

func resolveUser(uid string) string {
	if uid == "" {
		return ""
	}
	u, err := user.LookupId(uid)
	if err != nil || u == nil || u.Username == "" {
		return uid
	}
	return u.Username
}

func readMemTotalBytes() int64 {
	line := readFirstLine("/proc/meminfo")
	// meminfo first line is typically MemTotal; fall back to a tiny scan if needed.
	if strings.HasPrefix(line, "MemTotal:") {
		fields := strings.Fields(line)
		if len(fields) >= 2 {
			kb, _ := strconv.ParseInt(fields[1], 10, 64)
			return kb * 1024
		}
	}
	b, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return 0
	}
	for l := range strings.SplitSeq(string(b), "\n") {
		if !strings.HasPrefix(l, "MemTotal:") {
			continue
		}
		fields := strings.Fields(l)
		if len(fields) < 2 {
			return 0
		}
		kb, _ := strconv.ParseInt(fields[1], 10, 64)
		return kb * 1024
	}
	return 0
}

func truncate(s string, n int) string {
	if n <= 0 || len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}
