package machine

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// GPUMetrics describes a graphics device detected on the host.
type GPUMetrics struct {
	Index    int    `json:"index"`
	Name     string `json:"name"`
	Vendor   string `json:"vendor"`
	VendorID string `json:"vendor_id"`
	DeviceID string `json:"device_id"`
	PCISlot  string `json:"pci_slot"`
	Driver   string `json:"driver"`
	DRMCard  string `json:"drm_card"`
	BootVGA  bool   `json:"boot_vga"`
	Brand    string `json:"brand"` // nvidia | amd | intel | other

	MemoryTotalBytes int64   `json:"memory_total_bytes"`
	MemoryUsedBytes  int64   `json:"memory_used_bytes"`
	MemoryFreeBytes  int64   `json:"memory_free_bytes"`
	MemoryTotalHuman string  `json:"memory_total_human"`
	MemoryUsedHuman  string  `json:"memory_used_human"`
	MemoryFreeHuman  string  `json:"memory_free_human"`
	MemoryPercent    float64 `json:"memory_percent"`

	UtilizationPercent float64 `json:"utilization_percent"`
	MemoryUtilPercent  float64 `json:"memory_util_percent"`

	TemperatureC    float64 `json:"temperature_c"`
	PowerDrawW      float64 `json:"power_draw_w"`
	PowerLimitW     float64 `json:"power_limit_w"`
	FanSpeedPercent float64 `json:"fan_speed_percent"`

	ClockGraphicsMHz    int `json:"clock_graphics_mhz"`
	ClockMaxGraphicsMHz int `json:"clock_max_graphics_mhz"`
	ClockMinGraphicsMHz int `json:"clock_min_graphics_mhz"`
	ClockMemoryMHz      int `json:"clock_memory_mhz"`

	UUID              string `json:"uuid"`
	DriverVersion     string `json:"driver_version"`
	CUDAVersion       string `json:"cuda_version,omitempty"`
	ComputeCapability string `json:"compute_capability"`

	Connectors []GPUConnector `json:"connectors"`
}

// GPUConnector is a DRM display connector attached to a GPU.
type GPUConnector struct {
	Name    string `json:"name"`
	Type    string `json:"type"`
	Status  string `json:"status"`
	Enabled string `json:"enabled"`
}

// readGPUMetrics discovers GPUs via nvidia-smi (when present) and DRM/sysfs.
func readGPUMetrics() []GPUMetrics {
	byPCI := map[string]*GPUMetrics{}
	var order []string

	upsert := func(g GPUMetrics) {
		key := g.PCISlot
		if key == "" {
			key = g.UUID
		}
		if key == "" {
			key = g.DRMCard
		}
		if key == "" {
			key = fmt.Sprintf("gpu-%d", len(order))
		}
		if existing, ok := byPCI[key]; ok {
			mergeGPU(existing, g)
			return
		}
		cp := g
		byPCI[key] = &cp
		order = append(order, key)
	}

	for _, g := range readNvidiaGPUs() {
		upsert(g)
	}
	for _, g := range readDRMGPUs() {
		upsert(g)
	}

	out := make([]GPUMetrics, 0, len(order))
	for i, key := range order {
		g := byPCI[key]
		g.Index = i
		finalizeGPU(g)
		out = append(out, *g)
	}
	return out
}

func mergeGPU(dst *GPUMetrics, src GPUMetrics) {
	if dst.Name == "" || looksGenericGPUName(dst.Name) {
		if src.Name != "" {
			dst.Name = src.Name
		}
	}
	if dst.Vendor == "" {
		dst.Vendor = src.Vendor
	}
	if dst.VendorID == "" {
		dst.VendorID = src.VendorID
	}
	if dst.DeviceID == "" {
		dst.DeviceID = src.DeviceID
	}
	if dst.PCISlot == "" {
		dst.PCISlot = src.PCISlot
	}
	if dst.Driver == "" {
		dst.Driver = src.Driver
	}
	if dst.DRMCard == "" {
		dst.DRMCard = src.DRMCard
	}
	if dst.Brand == "" || dst.Brand == "other" {
		dst.Brand = src.Brand
	}
	if src.BootVGA {
		dst.BootVGA = true
	}
	if dst.MemoryTotalBytes == 0 {
		dst.MemoryTotalBytes = src.MemoryTotalBytes
		dst.MemoryUsedBytes = src.MemoryUsedBytes
		dst.MemoryFreeBytes = src.MemoryFreeBytes
	}
	if dst.UtilizationPercent == 0 {
		dst.UtilizationPercent = src.UtilizationPercent
	}
	if dst.MemoryUtilPercent == 0 {
		dst.MemoryUtilPercent = src.MemoryUtilPercent
	}
	if dst.TemperatureC == 0 {
		dst.TemperatureC = src.TemperatureC
	}
	if dst.PowerDrawW == 0 {
		dst.PowerDrawW = src.PowerDrawW
	}
	if dst.PowerLimitW == 0 {
		dst.PowerLimitW = src.PowerLimitW
	}
	if dst.FanSpeedPercent == 0 {
		dst.FanSpeedPercent = src.FanSpeedPercent
	}
	if dst.ClockGraphicsMHz == 0 {
		dst.ClockGraphicsMHz = src.ClockGraphicsMHz
	}
	if dst.ClockMaxGraphicsMHz == 0 {
		dst.ClockMaxGraphicsMHz = src.ClockMaxGraphicsMHz
	}
	if dst.ClockMinGraphicsMHz == 0 {
		dst.ClockMinGraphicsMHz = src.ClockMinGraphicsMHz
	}
	if dst.ClockMemoryMHz == 0 {
		dst.ClockMemoryMHz = src.ClockMemoryMHz
	}
	if dst.UUID == "" {
		dst.UUID = src.UUID
	}
	if dst.DriverVersion == "" {
		dst.DriverVersion = src.DriverVersion
	}
	if dst.CUDAVersion == "" {
		dst.CUDAVersion = src.CUDAVersion
	}
	if dst.ComputeCapability == "" {
		dst.ComputeCapability = src.ComputeCapability
	}
	if len(dst.Connectors) == 0 {
		dst.Connectors = src.Connectors
	}
}

func looksGenericGPUName(name string) bool {
	n := strings.ToLower(strings.TrimSpace(name))
	return n == "" ||
		strings.HasPrefix(n, "pci ") ||
		n == "vga compatible controller" ||
		n == "3d controller" ||
		n == "display controller"
}

func finalizeGPU(g *GPUMetrics) {
	if g.MemoryFreeBytes == 0 && g.MemoryTotalBytes > 0 && g.MemoryUsedBytes >= 0 {
		free := max(g.MemoryTotalBytes-g.MemoryUsedBytes, 0)
		g.MemoryFreeBytes = free
	}
	if g.MemoryTotalBytes > 0 {
		g.MemoryPercent = round2(float64(g.MemoryUsedBytes) / float64(g.MemoryTotalBytes) * 100)
		g.MemoryTotalHuman = humanBytes(g.MemoryTotalBytes)
		g.MemoryUsedHuman = humanBytes(g.MemoryUsedBytes)
		g.MemoryFreeHuman = humanBytes(g.MemoryFreeBytes)
	}
	if g.Name == "" {
		g.Name = gpuDisplayName(g.VendorID, g.DeviceID, g.Vendor)
	}
	if g.Brand == "" {
		g.Brand = gpuBrand(g.VendorID, g.Driver)
	}
	if g.Vendor == "" {
		g.Vendor = gpuVendorName(g.VendorID)
	}
}

func readNvidiaGPUs() []GPUMetrics {
	path, err := exec.LookPath("nvidia-smi")
	if err != nil {
		return nil
	}

	query := strings.Join([]string{
		"index",
		"name",
		"uuid",
		"driver_version",
		"pci.bus_id",
		"pci.device_id",
		"temperature.gpu",
		"utilization.gpu",
		"utilization.memory",
		"memory.total",
		"memory.used",
		"memory.free",
		"power.draw",
		"power.limit",
		"clocks.current.graphics",
		"clocks.max.graphics",
		"clocks.current.memory",
		"fan.speed",
		"compute_cap",
	}, ",")

	cmd := exec.Command(path,
		"--query-gpu="+query,
		"--format=csv,noheader,nounits",
	)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	done := make(chan error, 1)
	go func() { done <- cmd.Run() }()

	select {
	case err := <-done:
		if err != nil {
			return nil
		}
	case <-time.After(2 * time.Second):
		_ = cmd.Process.Kill()
		return nil
	}

	cudaVersion := readNvidiaCUDAVersion(path)

	var out []GPUMetrics
	sc := bufio.NewScanner(bytes.NewReader(stdout.Bytes()))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		fields := splitCSVLine(line)
		if len(fields) < 19 {
			continue
		}
		memTotal := parseFloatMiBToBytes(fields[9])
		memUsed := parseFloatMiBToBytes(fields[10])
		memFree := parseFloatMiBToBytes(fields[11])
		pciSlot := normalizePCIBusID(fields[4])
		deviceID := strings.TrimPrefix(strings.ToLower(strings.TrimSpace(fields[5])), "0x")

		g := GPUMetrics{
			Name:                strings.TrimSpace(fields[1]),
			UUID:                strings.TrimSpace(fields[2]),
			DriverVersion:       strings.TrimSpace(fields[3]),
			PCISlot:             pciSlot,
			VendorID:            "10de",
			DeviceID:            deviceID,
			Vendor:              "NVIDIA",
			Brand:               "nvidia",
			Driver:              "nvidia",
			CUDAVersion:         cudaVersion,
			TemperatureC:        parseFloatOrZero(fields[6]),
			UtilizationPercent:  parseFloatOrZero(fields[7]),
			MemoryUtilPercent:   parseFloatOrZero(fields[8]),
			MemoryTotalBytes:    memTotal,
			MemoryUsedBytes:     memUsed,
			MemoryFreeBytes:     memFree,
			PowerDrawW:          parseFloatOrZero(fields[12]),
			PowerLimitW:         parseFloatOrZero(fields[13]),
			ClockGraphicsMHz:    parseIntOrZero(fields[14]),
			ClockMaxGraphicsMHz: parseIntOrZero(fields[15]),
			ClockMemoryMHz:      parseIntOrZero(fields[16]),
			FanSpeedPercent:     parseFloatOrZero(fields[17]),
			ComputeCapability:   strings.TrimSpace(fields[18]),
		}
		out = append(out, g)
	}
	return out
}

func readNvidiaCUDAVersion(nvidiaSMI string) string {
	cmd := exec.Command(nvidiaSMI)
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stdout
	done := make(chan error, 1)
	go func() { done <- cmd.Run() }()
	select {
	case <-done:
	case <-time.After(1500 * time.Millisecond):
		_ = cmd.Process.Kill()
		return ""
	}
	// Header line typically contains: CUDA Version: 12.4
	for line := range strings.SplitSeq(stdout.String(), "\n") {
		if _, after, ok := strings.Cut(line, "CUDA Version:"); ok {
			return strings.TrimSpace(after)
		}
	}
	return ""
}

func readDRMGPUs() []GPUMetrics {
	entries, err := os.ReadDir("/sys/class/drm")
	if err != nil {
		return nil
	}

	var out []GPUMetrics
	for _, ent := range entries {
		name := ent.Name()
		// Only primary DRM cards: card0, card1, … — not card0-DP-1.
		if !strings.HasPrefix(name, "card") {
			continue
		}
		rest := strings.TrimPrefix(name, "card")
		if rest == "" {
			continue
		}
		if _, err := strconv.Atoi(rest); err != nil {
			continue
		}

		cardPath := filepath.Join("/sys/class/drm", name)
		devPath := filepath.Join(cardPath, "device")
		if st, err := os.Stat(devPath); err != nil || !st.IsDir() {
			continue
		}

		vendorID := strings.TrimPrefix(strings.ToLower(readFirstLine(filepath.Join(devPath, "vendor"))), "0x")
		deviceID := strings.TrimPrefix(strings.ToLower(readFirstLine(filepath.Join(devPath, "device"))), "0x")
		if vendorID == "" || deviceID == "" {
			continue
		}

		classHex := strings.TrimPrefix(strings.ToLower(readFirstLine(filepath.Join(devPath, "class"))), "0x")
		// Display controllers: VGA (0300xx) or 3D (0302xx).
		if len(classHex) >= 4 {
			base := classHex
			if len(base) > 4 {
				base = base[:4]
			}
			if base != "0300" && base != "0302" {
				continue
			}
		}

		pciSlot := ""
		if uevent := readFileLimit(filepath.Join(devPath, "uevent"), 4096); uevent != "" {
			for line := range strings.SplitSeq(uevent, "\n") {
				if after, ok := strings.CutPrefix(line, "PCI_SLOT_NAME="); ok {
					pciSlot = after
					break
				}
			}
		}

		driver := ""
		if link, err := os.Readlink(filepath.Join(devPath, "driver")); err == nil {
			driver = filepath.Base(link)
		}

		brand := gpuBrand(vendorID, driver)
		vendor := gpuVendorName(vendorID)
		nameStr := lookupPCIName(vendorID, deviceID)
		if nameStr == "" {
			nameStr = gpuDisplayName(vendorID, deviceID, vendor)
		}

		g := GPUMetrics{
			Name:     nameStr,
			Vendor:   vendor,
			VendorID: vendorID,
			DeviceID: deviceID,
			PCISlot:  pciSlot,
			Driver:   driver,
			DRMCard:  name,
			BootVGA:  readFirstLine(filepath.Join(devPath, "boot_vga")) == "1",
			Brand:    brand,
		}

		// AMD / some discrete cards expose VRAM + busy % under the PCI device.
		if v := readSysInt64(filepath.Join(devPath, "mem_info_vram_total")); v > 0 {
			g.MemoryTotalBytes = v
			g.MemoryUsedBytes = readSysInt64(filepath.Join(devPath, "mem_info_vram_used"))
		}
		if busy := readSysFloat(filepath.Join(devPath, "gpu_busy_percent")); busy >= 0 {
			g.UtilizationPercent = busy
		}

		// Intel i915 frequency nodes live on the DRM card directory.
		g.ClockGraphicsMHz = readSysInt(filepath.Join(cardPath, "gt_cur_freq_mhz"))
		if g.ClockGraphicsMHz == 0 {
			g.ClockGraphicsMHz = readSysInt(filepath.Join(cardPath, "gt_act_freq_mhz"))
		}
		if g.ClockGraphicsMHz == 0 {
			g.ClockGraphicsMHz = readSysInt(filepath.Join(cardPath, "gt", "gt0", "rps_cur_freq_mhz"))
		}
		g.ClockMaxGraphicsMHz = readSysInt(filepath.Join(cardPath, "gt_max_freq_mhz"))
		if g.ClockMaxGraphicsMHz == 0 {
			g.ClockMaxGraphicsMHz = readSysInt(filepath.Join(cardPath, "gt_RP0_freq_mhz"))
		}
		if g.ClockMaxGraphicsMHz == 0 {
			g.ClockMaxGraphicsMHz = readSysInt(filepath.Join(cardPath, "gt", "gt0", "rps_max_freq_mhz"))
		}
		g.ClockMinGraphicsMHz = readSysInt(filepath.Join(cardPath, "gt_min_freq_mhz"))
		if g.ClockMinGraphicsMHz == 0 {
			g.ClockMinGraphicsMHz = readSysInt(filepath.Join(cardPath, "gt_RPn_freq_mhz"))
		}
		if g.ClockMinGraphicsMHz == 0 {
			g.ClockMinGraphicsMHz = readSysInt(filepath.Join(cardPath, "gt", "gt0", "rps_min_freq_mhz"))
		}

		// Intel: derive utilization from RC6 residency between samples.
		if g.UtilizationPercent == 0 && (brand == "intel" || driver == "i915" || driver == "xe") {
			if busy := readIntelBusyPercent(cardPath); busy >= 0 {
				g.UtilizationPercent = busy
			}
		}

		g.TemperatureC = readGPUTempC(devPath)
		g.Connectors = readDRMConnectors(cardPath, name)

		out = append(out, g)
	}
	return out
}

type gpuRC6Sample struct {
	at          time.Time
	residencyMs int64
}

var (
	gpuRC6Mu  sync.Mutex
	gpuRC6Map = map[string]gpuRC6Sample{}
)

// readIntelBusyPercent estimates GPU busy % from RC6 power-gating residency.
// Returns -1 when a reading is not yet available (first sample).
func readIntelBusyPercent(cardPath string) float64 {
	var residency int64 = -1
	for _, p := range []string{
		filepath.Join(cardPath, "gt", "gt0", "rc6_residency_ms"),
		filepath.Join(cardPath, "power", "rc6_residency_ms"),
	} {
		raw := strings.TrimSpace(readFirstLine(p))
		if raw == "" {
			continue
		}
		v, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			continue
		}
		residency = v
		break
	}
	if residency < 0 {
		return -1
	}

	now := time.Now()
	gpuRC6Mu.Lock()
	defer gpuRC6Mu.Unlock()

	prev, ok := gpuRC6Map[cardPath]
	gpuRC6Map[cardPath] = gpuRC6Sample{at: now, residencyMs: residency}
	if !ok {
		return -1
	}

	dtMs := now.Sub(prev.at).Seconds() * 1000
	if dtMs < 50 {
		return -1
	}
	dRC6 := float64(residency - prev.residencyMs)
	if dRC6 < 0 {
		return -1
	}
	idleFrac := dRC6 / dtMs
	if idleFrac > 1 {
		idleFrac = 1
	}
	busy := (1 - idleFrac) * 100
	if busy < 0 {
		busy = 0
	}
	if busy > 100 {
		busy = 100
	}
	return round2(busy)
}

func readDRMConnectors(cardPath, cardName string) []GPUConnector {
	entries, err := os.ReadDir(cardPath)
	if err != nil {
		return nil
	}
	prefix := cardName + "-"
	var out []GPUConnector
	for _, ent := range entries {
		name := ent.Name()
		if !strings.HasPrefix(name, prefix) {
			continue
		}
		connPath := filepath.Join(cardPath, name)
		status := readFirstLine(filepath.Join(connPath, "status"))
		enabled := readFirstLine(filepath.Join(connPath, "enabled"))
		connType := strings.TrimPrefix(name, prefix)
		// Strip trailing -N index for a cleaner type label (HDMI-A-1 → HDMI-A).
		typeLabel := connType
		if i := strings.LastIndex(connType, "-"); i > 0 {
			if _, err := strconv.Atoi(connType[i+1:]); err == nil {
				typeLabel = connType[:i]
			}
		}
		out = append(out, GPUConnector{
			Name:    connType,
			Type:    typeLabel,
			Status:  status,
			Enabled: enabled,
		})
	}
	return out
}

func readGPUTempC(devPath string) float64 {
	hwmonRoot := filepath.Join(devPath, "hwmon")
	entries, err := os.ReadDir(hwmonRoot)
	if err != nil {
		return 0
	}
	for _, ent := range entries {
		if !strings.HasPrefix(ent.Name(), "hwmon") {
			continue
		}
		dir := filepath.Join(hwmonRoot, ent.Name())
		// Prefer temp1_input (millidegrees).
		for _, name := range []string{"temp1_input", "temp2_input", "temp3_input"} {
			raw := readFirstLine(filepath.Join(dir, name))
			if raw == "" {
				continue
			}
			v, err := strconv.ParseFloat(raw, 64)
			if err != nil {
				continue
			}
			if v > 200 { // millidegrees C
				return round2(v / 1000)
			}
			return round2(v)
		}
	}
	return 0
}

func gpuBrand(vendorID, driver string) string {
	switch strings.ToLower(vendorID) {
	case "10de":
		return "nvidia"
	case "1002", "1022":
		return "amd"
	case "8086":
		return "intel"
	}
	d := strings.ToLower(driver)
	switch {
	case strings.Contains(d, "nvidia"):
		return "nvidia"
	case strings.Contains(d, "amdgpu") || strings.Contains(d, "radeon"):
		return "amd"
	case strings.Contains(d, "i915") || strings.Contains(d, "xe"):
		return "intel"
	default:
		return "other"
	}
}

func gpuVendorName(vendorID string) string {
	switch strings.ToLower(vendorID) {
	case "10de":
		return "NVIDIA"
	case "1002", "1022":
		return "AMD"
	case "8086":
		return "Intel"
	case "1a03":
		return "ASPEED"
	case "15ad":
		return "VMware"
	case "1234":
		return "QEMU"
	case "1414":
		return "Microsoft"
	default:
		if vendorID == "" {
			return ""
		}
		return "PCI " + strings.ToUpper(vendorID)
	}
}

func gpuDisplayName(vendorID, deviceID, vendor string) string {
	if name := lookupPCIName(vendorID, deviceID); name != "" {
		return name
	}
	if known := knownGPUName(vendorID, deviceID); known != "" {
		return known
	}
	v := vendor
	if v == "" {
		v = gpuVendorName(vendorID)
	}
	if deviceID != "" {
		return fmt.Sprintf("%s Graphics (%s)", v, strings.ToUpper(deviceID))
	}
	return v + " Graphics"
}

func knownGPUName(vendorID, deviceID string) string {
	key := strings.ToLower(vendorID) + ":" + strings.ToLower(deviceID)
	// Small fallback table when pci.ids is unavailable (common iGPUs / virt).
	names := map[string]string{
		"8086:0102": "HD Graphics 2000",
		"8086:0112": "HD Graphics 3000",
		"8086:0122": "HD Graphics 3000",
		"8086:0152": "HD Graphics 2500",
		"8086:0162": "HD Graphics 4000",
		"8086:0412": "HD Graphics 4600",
		"8086:0416": "HD Graphics 4600",
		"8086:0a16": "HD Graphics 4400",
		"8086:0d26": "HD Graphics 5000",
		"8086:1616": "HD Graphics 5500",
		"8086:1912": "HD Graphics 530",
		"8086:1916": "HD Graphics 520",
		"8086:191b": "HD Graphics 530",
		"8086:3e92": "UHD Graphics 630",
		"8086:3e9b": "UHD Graphics 630",
		"8086:9a49": "Iris Xe Graphics",
		"8086:46a6": "Iris Xe Graphics",
		"8086:a7a0": "UHD Graphics",
		"1234:1111": "QEMU Standard VGA",
		"15ad:0405": "VMware SVGA",
		"1a03:2000": "ASPEED Graphics",
	}
	return names[key]
}

func lookupPCIName(vendorID, deviceID string) string {
	vendorID = strings.ToLower(strings.TrimPrefix(vendorID, "0x"))
	deviceID = strings.ToLower(strings.TrimPrefix(deviceID, "0x"))
	if vendorID == "" || deviceID == "" {
		return ""
	}
	paths := []string{
		"/usr/share/misc/pci.ids",
		"/usr/share/hwdata/pci.ids",
		"/usr/share/pci.ids",
	}
	for _, path := range paths {
		if name := lookupPCINameInFile(path, vendorID, deviceID); name != "" {
			return name
		}
	}
	return ""
}

func lookupPCINameInFile(path, vendorID, deviceID string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()

	inVendor := false
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "\t\t") {
			continue
		}
		if !strings.HasPrefix(line, "\t") {
			fields := strings.Fields(line)
			if len(fields) < 2 {
				inVendor = false
				continue
			}
			inVendor = strings.EqualFold(fields[0], vendorID)
			continue
		}
		if !inVendor {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		if strings.EqualFold(fields[0], deviceID) {
			return strings.TrimSpace(strings.TrimPrefix(line, "\t"+fields[0]))
		}
	}
	return ""
}

func normalizePCIBusID(bus string) string {
	bus = strings.TrimSpace(bus)
	if bus == "" {
		return ""
	}
	// nvidia-smi: 00000000:01:00.0 → 0000:01:00.0
	parts := strings.Split(bus, ":")
	if len(parts) == 3 && len(parts[0]) > 4 {
		parts[0] = parts[0][len(parts[0])-4:]
		return strings.Join(parts, ":")
	}
	return bus
}

func splitCSVLine(line string) []string {
	parts := strings.Split(line, ",")
	out := make([]string, len(parts))
	for i, p := range parts {
		out[i] = strings.TrimSpace(p)
	}
	return out
}

func parseFloatOrZero(s string) float64 {
	s = strings.TrimSpace(s)
	if s == "" || s == "[N/A]" || s == "N/A" || s == "[Not Supported]" {
		return 0
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0
	}
	return v
}

func parseIntOrZero(s string) int {
	return int(parseFloatOrZero(s))
}

func parseFloatMiBToBytes(s string) int64 {
	v := parseFloatOrZero(s)
	if v <= 0 {
		return 0
	}
	return int64(v * 1024 * 1024)
}

func readSysInt64(path string) int64 {
	s := strings.TrimSpace(readFirstLine(path))
	if s == "" {
		return 0
	}
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0
	}
	return v
}

func readSysInt(path string) int {
	return int(readSysInt64(path))
}

func readSysFloat(path string) float64 {
	s := strings.TrimSpace(readFirstLine(path))
	if s == "" {
		return -1
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return -1
	}
	return v
}
