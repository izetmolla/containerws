package machine

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Metrics is a live resource snapshot suitable for dashboard polling.
type Metrics struct {
	CollectedAt   time.Time      `json:"collected_at"`
	Host          HostMetrics    `json:"host"`
	CPU           CPUMetrics     `json:"cpu"`
	Memory        MemoryMetrics  `json:"memory"`
	Disks         []DiskMetrics  `json:"disks"`
	Network       NetworkMetrics `json:"network"`
	GPUs          []GPUMetrics   `json:"gpus"`
	UptimeSeconds float64        `json:"uptime_seconds"`
}

type HostMetrics struct {
	Hostname         string `json:"hostname"`
	PrimaryIP        string `json:"primary_ip"`
	DefaultIface     string `json:"default_iface"`
	OS               string `json:"os"`
	Kernel           string `json:"kernel"`
	Distro           string `json:"distro"`
	DistroVersion    string `json:"distro_version"`
	Arch             string `json:"arch"`
	Virtualization   string `json:"virtualization"`
	IsContainerized  bool   `json:"is_containerized"`
	IsVirtualMachine bool   `json:"is_virtual_machine"`
}

type CPUMetrics struct {
	Cores          int       `json:"cores"`
	Model          string    `json:"model"`
	UsagePercent   float64   `json:"usage_percent"`
	Load1          float64   `json:"load1"`
	Load5          float64   `json:"load5"`
	Load15         float64   `json:"load15"`
	PerCorePercent []float64 `json:"per_core_percent"`
}

type MemoryMetrics struct {
	TotalBytes     int64   `json:"total_bytes"`
	UsedBytes      int64   `json:"used_bytes"`
	AvailableBytes int64   `json:"available_bytes"`
	FreeBytes      int64   `json:"free_bytes"`
	BuffersBytes   int64   `json:"buffers_bytes"`
	CachedBytes    int64   `json:"cached_bytes"`
	SwapTotalBytes int64   `json:"swap_total_bytes"`
	SwapUsedBytes  int64   `json:"swap_used_bytes"`
	UsedPercent    float64 `json:"used_percent"`
	SwapPercent    float64 `json:"swap_percent"`
	TotalHuman     string  `json:"total_human"`
	UsedHuman      string  `json:"used_human"`
	AvailableHuman string  `json:"available_human"`
	CachedHuman    string  `json:"cached_human"`
	BuffersHuman   string  `json:"buffers_human"`
	SwapTotalHuman string  `json:"swap_total_human"`
	SwapUsedHuman  string  `json:"swap_used_human"`
}

type DiskMetrics struct {
	Mount       string  `json:"mount"`
	Device      string  `json:"device"`
	FSType      string  `json:"fstype"`
	TotalBytes  int64   `json:"total_bytes"`
	UsedBytes   int64   `json:"used_bytes"`
	FreeBytes   int64   `json:"free_bytes"`
	UsedPercent float64 `json:"used_percent"`
	TotalHuman  string  `json:"total_human"`
	UsedHuman   string  `json:"used_human"`
	FreeHuman   string  `json:"free_human"`
}

type NetworkMetrics struct {
	RxBytesTotal int64          `json:"rx_bytes_total"`
	TxBytesTotal int64          `json:"tx_bytes_total"`
	RxRateBps    float64        `json:"rx_rate_bps"`
	TxRateBps    float64        `json:"tx_rate_bps"`
	RxRateHuman  string         `json:"rx_rate_human"`
	TxRateHuman  string         `json:"tx_rate_human"`
	Interfaces   []NetworkIface `json:"interfaces"`
}

type NetworkIface struct {
	Name        string  `json:"name"`
	RxBytes     int64   `json:"rx_bytes"`
	TxBytes     int64   `json:"tx_bytes"`
	RxPackets   int64   `json:"rx_packets"`
	TxPackets   int64   `json:"tx_packets"`
	RxErrors    int64   `json:"rx_errors"`
	TxErrors    int64   `json:"tx_errors"`
	RxDropped   int64   `json:"rx_dropped"`
	TxDropped   int64   `json:"tx_dropped"`
	RxRateBps   float64 `json:"rx_rate_bps"`
	TxRateBps   float64 `json:"tx_rate_bps"`
	RxRateHuman string  `json:"rx_rate_human"`
	TxRateHuman string  `json:"tx_rate_human"`
	RxHuman     string  `json:"rx_human"`
	TxHuman     string  `json:"tx_human"`
}

type cpuSample struct {
	total uint64
	idle  uint64
}

type netSample struct {
	rx, tx int64
}

type metricsPrev struct {
	mu        sync.Mutex
	at        time.Time
	cpu       cpuSample
	perCore   []cpuSample
	netTotal  netSample
	netIfaces map[string]netSample
}

var livePrev metricsPrev

// CollectMetrics reads /proc and returns a live resource snapshot.
// CPU and network rates are derived from the previous sample when available.
func CollectMetrics() Metrics {
	now := time.Now().UTC()
	snap := Detect()

	mem := readMemoryMetrics()
	cpuNow, perNow := readCPUSamples()
	load1, load5, load15 := readLoadAvg()
	netIfaces := readNetworkIfaces()

	var rxTotal, txTotal int64
	for _, iface := range netIfaces {
		if iface.Name == "lo" {
			continue
		}
		rxTotal += iface.RxBytes
		txTotal += iface.TxBytes
	}

	m := Metrics{
		CollectedAt: now,
		Host: HostMetrics{
			Hostname:         snap.Hostname,
			PrimaryIP:        snap.PrimaryIP,
			DefaultIface:     snap.DefaultIface,
			OS:               snap.OS,
			Kernel:           snap.Kernel,
			Distro:           snap.Distro,
			DistroVersion:    snap.DistroVersion,
			Arch:             snap.Arch,
			Virtualization:   snap.Virtualization,
			IsContainerized:  snap.IsContainerized,
			IsVirtualMachine: snap.IsVirtualMachine,
		},
		CPU: CPUMetrics{
			Cores:  snap.CPUCores,
			Model:  snap.CPUModel,
			Load1:  load1,
			Load5:  load5,
			Load15: load15,
		},
		Memory:        mem,
		Disks:         readDiskMetrics(),
		GPUs:          readGPUMetrics(),
		UptimeSeconds: readUptimeSeconds(),
		Network: NetworkMetrics{
			RxBytesTotal: rxTotal,
			TxBytesTotal: txTotal,
			Interfaces:   netIfaces,
		},
	}

	livePrev.mu.Lock()
	defer livePrev.mu.Unlock()

	elapsed := now.Sub(livePrev.at).Seconds()
	if livePrev.at.IsZero() || elapsed < 0.2 {
		// Bootstrap: take a short second sample so the first response has rates.
		time.Sleep(120 * time.Millisecond)
		cpu2, per2 := readCPUSamples()
		m.CPU.UsagePercent = cpuUsagePercent(cpuNow, cpu2)
		m.CPU.PerCorePercent = perCoreUsage(perNow, per2)
		net2 := readNetworkIfaces()
		applyNetworkRates(&m, netIfaces, net2, 0.12)
		livePrev.at = time.Now().UTC()
		livePrev.cpu = cpu2
		livePrev.perCore = per2
		livePrev.netTotal = netSample{rx: m.Network.RxBytesTotal, tx: m.Network.TxBytesTotal}
		livePrev.netIfaces = ifaceSampleMap(net2)
		return m
	}

	m.CPU.UsagePercent = cpuUsagePercent(livePrev.cpu, cpuNow)
	m.CPU.PerCorePercent = perCoreUsage(livePrev.perCore, perNow)

	dt := elapsed
	if dt < 0.01 {
		dt = 0.01
	}
	m.Network.RxRateBps = float64(rxTotal-livePrev.netTotal.rx) / dt
	m.Network.TxRateBps = float64(txTotal-livePrev.netTotal.tx) / dt
	if m.Network.RxRateBps < 0 {
		m.Network.RxRateBps = 0
	}
	if m.Network.TxRateBps < 0 {
		m.Network.TxRateBps = 0
	}
	m.Network.RxRateHuman = humanRate(m.Network.RxRateBps)
	m.Network.TxRateHuman = humanRate(m.Network.TxRateBps)

	for i := range m.Network.Interfaces {
		iface := &m.Network.Interfaces[i]
		prev, ok := livePrev.netIfaces[iface.Name]
		if !ok {
			continue
		}
		iface.RxRateBps = float64(iface.RxBytes-prev.rx) / dt
		iface.TxRateBps = float64(iface.TxBytes-prev.tx) / dt
		if iface.RxRateBps < 0 {
			iface.RxRateBps = 0
		}
		if iface.TxRateBps < 0 {
			iface.TxRateBps = 0
		}
		iface.RxRateHuman = humanRate(iface.RxRateBps)
		iface.TxRateHuman = humanRate(iface.TxRateBps)
	}

	livePrev.at = now
	livePrev.cpu = cpuNow
	livePrev.perCore = perNow
	livePrev.netTotal = netSample{rx: rxTotal, tx: txTotal}
	livePrev.netIfaces = ifaceSampleMap(netIfaces)
	return m
}

func applyNetworkRates(m *Metrics, before, after []NetworkIface, dt float64) {
	if dt <= 0 {
		dt = 0.12
	}
	bMap := ifaceSampleMap(before)
	var rxTotal, txTotal int64
	for i := range after {
		iface := &after[i]
		if iface.Name != "lo" {
			rxTotal += iface.RxBytes
			txTotal += iface.TxBytes
		}
		prev, ok := bMap[iface.Name]
		if !ok {
			continue
		}
		iface.RxRateBps = float64(iface.RxBytes-prev.rx) / dt
		iface.TxRateBps = float64(iface.TxBytes-prev.tx) / dt
		if iface.RxRateBps < 0 {
			iface.RxRateBps = 0
		}
		if iface.TxRateBps < 0 {
			iface.TxRateBps = 0
		}
		iface.RxRateHuman = humanRate(iface.RxRateBps)
		iface.TxRateHuman = humanRate(iface.TxRateBps)
	}
	prevRx, prevTx := int64(0), int64(0)
	for name, s := range bMap {
		if name == "lo" {
			continue
		}
		prevRx += s.rx
		prevTx += s.tx
	}
	m.Network.RxBytesTotal = rxTotal
	m.Network.TxBytesTotal = txTotal
	m.Network.RxRateBps = float64(rxTotal-prevRx) / dt
	m.Network.TxRateBps = float64(txTotal-prevTx) / dt
	if m.Network.RxRateBps < 0 {
		m.Network.RxRateBps = 0
	}
	if m.Network.TxRateBps < 0 {
		m.Network.TxRateBps = 0
	}
	m.Network.RxRateHuman = humanRate(m.Network.RxRateBps)
	m.Network.TxRateHuman = humanRate(m.Network.TxRateBps)
	m.Network.Interfaces = after
}

func ifaceSampleMap(ifaces []NetworkIface) map[string]netSample {
	out := make(map[string]netSample, len(ifaces))
	for _, iface := range ifaces {
		out[iface.Name] = netSample{rx: iface.RxBytes, tx: iface.TxBytes}
	}
	return out
}

func readMemoryMetrics() MemoryMetrics {
	vals := map[string]int64{}
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return MemoryMetrics{}
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) < 2 {
			continue
		}
		key := strings.TrimSuffix(fields[0], ":")
		kb, _ := strconv.ParseInt(fields[1], 10, 64)
		vals[key] = kb * 1024
	}

	total := vals["MemTotal"]
	free := vals["MemFree"]
	available := vals["MemAvailable"]
	buffers := vals["Buffers"]
	cached := vals["Cached"] + vals["SReclaimable"]
	swapTotal := vals["SwapTotal"]
	swapFree := vals["SwapFree"]
	if available <= 0 {
		available = free + buffers + cached
	}
	used := max(total-available, 0)
	swapUsed := max(swapTotal-swapFree, 0)

	m := MemoryMetrics{
		TotalBytes:     total,
		UsedBytes:      used,
		AvailableBytes: available,
		FreeBytes:      free,
		BuffersBytes:   buffers,
		CachedBytes:    cached,
		SwapTotalBytes: swapTotal,
		SwapUsedBytes:  swapUsed,
		TotalHuman:     humanBytes(total),
		UsedHuman:      humanBytes(used),
		AvailableHuman: humanBytes(available),
		CachedHuman:    humanBytes(cached),
		BuffersHuman:   humanBytes(buffers),
		SwapTotalHuman: humanBytes(swapTotal),
		SwapUsedHuman:  humanBytes(swapUsed),
	}
	if total > 0 {
		m.UsedPercent = round2(float64(used) / float64(total) * 100)
	}
	if swapTotal > 0 {
		m.SwapPercent = round2(float64(swapUsed) / float64(swapTotal) * 100)
	}
	return m
}

func readCPUSamples() (all cpuSample, per []cpuSample) {
	f, err := os.Open("/proc/stat")
	if err != nil {
		return cpuSample{}, nil
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		if !strings.HasPrefix(line, "cpu") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 5 {
			continue
		}
		sample := parseCPUFields(fields[1:])
		if fields[0] == "cpu" {
			all = sample
			continue
		}
		if strings.HasPrefix(fields[0], "cpu") {
			per = append(per, sample)
		}
	}
	return all, per
}

func parseCPUFields(fields []string) cpuSample {
	var vals []uint64
	for _, f := range fields {
		v, _ := strconv.ParseUint(f, 10, 64)
		vals = append(vals, v)
	}
	var total uint64
	for _, v := range vals {
		total += v
	}
	var idle uint64
	if len(vals) > 3 {
		idle = vals[3]
	}
	if len(vals) > 4 {
		idle += vals[4] // iowait
	}
	return cpuSample{total: total, idle: idle}
}

func cpuUsagePercent(prev, cur cpuSample) float64 {
	dt := float64(cur.total - prev.total)
	di := float64(cur.idle - prev.idle)
	if dt <= 0 {
		return 0
	}
	usage := (1 - di/dt) * 100
	if usage < 0 {
		return 0
	}
	if usage > 100 {
		return 100
	}
	return round2(usage)
}

func perCoreUsage(prev, cur []cpuSample) []float64 {
	n := min(len(prev), len(cur))
	out := make([]float64, n)
	for i := range n {
		out[i] = cpuUsagePercent(prev[i], cur[i])
	}
	return out
}

func readLoadAvg() (float64, float64, float64) {
	line := readFirstLine("/proc/loadavg")
	fields := strings.Fields(line)
	if len(fields) < 3 {
		return 0, 0, 0
	}
	a, _ := strconv.ParseFloat(fields[0], 64)
	b, _ := strconv.ParseFloat(fields[1], 64)
	c, _ := strconv.ParseFloat(fields[2], 64)
	return a, b, c
}

func readUptimeSeconds() float64 {
	line := readFirstLine("/proc/uptime")
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return 0
	}
	v, _ := strconv.ParseFloat(fields[0], 64)
	return v
}

func lookupMountInfo(mount string) (device, fstype string) {
	f, err := os.Open("/proc/self/mountinfo")
	if err != nil {
		return "", ""
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		// mountinfo: ... - fstype source superopts
		parts := strings.Split(line, " - ")
		if len(parts) != 2 {
			continue
		}
		left := strings.Fields(parts[0])
		right := strings.Fields(parts[1])
		if len(left) < 5 || len(right) < 2 {
			continue
		}
		mp := left[4]
		if mp != mount {
			continue
		}
		return right[1], right[0]
	}
	_ = filepath.Clean(mount)
	return "", ""
}

func readNetworkIfaces() []NetworkIface {
	f, err := os.Open("/proc/net/dev")
	if err != nil {
		return nil
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	out := make([]NetworkIface, 0, 8)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if !strings.Contains(line, ":") || strings.HasPrefix(line, "Inter") || strings.HasPrefix(line, "face") {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		name := strings.TrimSpace(parts[0])
		fields := strings.Fields(parts[1])
		if len(fields) < 16 {
			continue
		}
		rx, _ := strconv.ParseInt(fields[0], 10, 64)
		rxPkts, _ := strconv.ParseInt(fields[1], 10, 64)
		rxErr, _ := strconv.ParseInt(fields[2], 10, 64)
		rxDrop, _ := strconv.ParseInt(fields[3], 10, 64)
		tx, _ := strconv.ParseInt(fields[8], 10, 64)
		txPkts, _ := strconv.ParseInt(fields[9], 10, 64)
		txErr, _ := strconv.ParseInt(fields[10], 10, 64)
		txDrop, _ := strconv.ParseInt(fields[11], 10, 64)
		out = append(out, NetworkIface{
			Name:        name,
			RxBytes:     rx,
			TxBytes:     tx,
			RxPackets:   rxPkts,
			TxPackets:   txPkts,
			RxErrors:    rxErr,
			TxErrors:    txErr,
			RxDropped:   rxDrop,
			TxDropped:   txDrop,
			RxHuman:     humanBytes(rx),
			TxHuman:     humanBytes(tx),
			RxRateHuman: "0 B/s",
			TxRateHuman: "0 B/s",
		})
	}
	return out
}

func humanRate(bps float64) string {
	if bps < 0 {
		bps = 0
	}
	units := []string{"B/s", "KB/s", "MB/s", "GB/s", "TB/s"}
	v := bps
	i := 0
	for v >= 1024 && i < len(units)-1 {
		v /= 1024
		i++
	}
	if i == 0 {
		return fmt.Sprintf("%.0f %s", v, units[i])
	}
	return fmt.Sprintf("%.1f %s", v, units[i])
}

func round2(v float64) float64 {
	return float64(int(v*100+0.5)) / 100
}
