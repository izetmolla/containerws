package machine

import (
	"bufio"
	"fmt"
	"maps"
	"net"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/izetmolla/containerws/models"
	"github.com/izetmolla/containerws/version"
	"github.com/spf13/viper"
)

// Snapshot is a point-in-time view of the host running this process.
type Snapshot struct {
	MachineID        string
	Hostname         string
	OS               string
	OSVersion        string
	Kernel           string
	Platform         string
	Distro           string
	DistroID         string
	DistroVersion    string
	Arch             string
	Processor        string
	CPUModel         string
	CPUCores         int
	MemoryTotal      int64
	MemoryHuman      string
	IPs              []string
	MACAddresses     []string
	PrimaryIP        string
	DefaultIface     string
	Type             models.ContainerType
	Virtualization   string
	Hypervisor       string
	ContainerRuntime string
	CloudProvider    string
	IsContainerized  bool
	IsVirtualMachine bool
	ProductName      string
	ProductUUID      string
	SysVendor        string
	BoardName        string
	AppVersion       string
	CommitSHA        string
	Metadata         map[string]any
}

// Detect collects OS, CPU, network, and virtualization identity for this host.
func Detect() Snapshot {
	hostname, _ := os.Hostname()
	kernel := readFirstLine("/proc/sys/kernel/osrelease")
	if kernel == "" {
		kernel = runtime.GOOS
	}

	distroID, distroName, distroVer := readOSRelease()
	cpuModel, cpuCores := readCPUInfo()
	memTotal := readMemTotal()
	ips, macs, primaryIP, iface := collectAddresses()

	productName := readDMI("product_name")
	productUUID := strings.TrimSpace(readDMI("product_uuid"))
	sysVendor := readDMI("sys_vendor")
	boardName := readDMI("board_name")

	virt, runtimeName, cloud, isCtr, isVM, ctype := detectVirtualization(sysVendor, productName)

	machineID := resolveMachineID(productUUID)
	processor := archLabel(runtime.GOARCH)
	if cpuModel != "" {
		processor = firstWord(cpuModel)
	}

	meta := map[string]any{
		"goos":            runtime.GOOS,
		"goarch":          runtime.GOARCH,
		"num_goroutine":   runtime.NumGoroutine(),
		"container_env":   os.Getenv("container"),
		"wsl_distro_name": os.Getenv("WSL_DISTRO_NAME"),
	}

	return Snapshot{
		MachineID:        machineID,
		Hostname:         hostname,
		OS:               runtime.GOOS,
		OSVersion:        kernel,
		Kernel:           kernel,
		Platform:         runtime.GOOS + "/" + runtime.GOARCH,
		Distro:           distroName,
		DistroID:         distroID,
		DistroVersion:    distroVer,
		Arch:             runtime.GOARCH,
		Processor:        processor,
		CPUModel:         cpuModel,
		CPUCores:         cpuCores,
		MemoryTotal:      memTotal,
		MemoryHuman:      humanBytes(memTotal),
		IPs:              ips,
		MACAddresses:     macs,
		PrimaryIP:        primaryIP,
		DefaultIface:     iface,
		Type:             ctype,
		Virtualization:   virt,
		Hypervisor:       firstNonEmpty(sysVendor, productName),
		ContainerRuntime: runtimeName,
		CloudProvider:    cloud,
		IsContainerized:  isCtr,
		IsVirtualMachine: isVM,
		ProductName:      productName,
		ProductUUID:      productUUID,
		SysVendor:        sysVendor,
		BoardName:        boardName,
		AppVersion:       version.Version,
		CommitSHA:        version.CommitSHA,
		Metadata:         meta,
	}
}

// ApplyToContainer copies snapshot fields onto a Container model (identity + hardware).
func (s Snapshot) ApplyToContainer(c *models.Container) {
	if c == nil {
		return
	}
	now := time.Now().UTC()
	if c.Name == "" {
		c.Name = defaultContainerName(s)
	}
	if c.Title == "" {
		c.Title = displayTitle(s)
	}
	c.MachineID = s.MachineID
	c.Hostname = s.Hostname
	c.OS = s.OS
	c.OSVersion = s.OSVersion
	c.Kernel = s.Kernel
	c.Platform = s.Platform
	c.Distro = s.Distro
	c.DistroID = s.DistroID
	c.DistroVersion = s.DistroVersion
	c.Arch = s.Arch
	c.Processor = s.Processor
	c.CPUModel = s.CPUModel
	c.CPUCores = s.CPUCores
	c.MemoryTotal = s.MemoryTotal
	c.MemoryHuman = s.MemoryHuman
	c.IPs = models.JSONBStringArray(s.IPs)
	c.MACAddresses = models.JSONBStringArray(s.MACAddresses)
	c.PrimaryIP = s.PrimaryIP
	c.DefaultIface = s.DefaultIface
	c.Type = s.Type
	c.Virtualization = s.Virtualization
	c.Hypervisor = s.Hypervisor
	c.ContainerRuntime = s.ContainerRuntime
	c.CloudProvider = s.CloudProvider
	c.IsContainerized = s.IsContainerized
	c.IsVirtualMachine = s.IsVirtualMachine
	c.ProductName = s.ProductName
	c.ProductUUID = s.ProductUUID
	c.SysVendor = s.SysVendor
	c.BoardName = s.BoardName
	c.AppVersion = s.AppVersion
	c.CommitSHA = s.CommitSHA
	c.BootedAt = &now
	c.LastSeenAt = &now
	c.IsActive = true
	if c.Metadata == nil {
		c.Metadata = models.JSONBAny{}
	}
	maps.Copy(c.Metadata, s.Metadata)
	c.Description = summarize(s)
}

func defaultContainerName(s Snapshot) string {
	if n := strings.TrimSpace(viper.GetString("CONTAINER_NAME")); n != "" {
		return n
	}
	if s.Hostname != "" {
		return s.Hostname
	}
	return "workspace"
}

func displayTitle(s Snapshot) string {
	parts := []string{}
	if s.Distro != "" {
		parts = append(parts, s.Distro)
		if s.DistroVersion != "" {
			parts[len(parts)-1] = s.Distro + " " + s.DistroVersion
		}
	}
	if s.Arch != "" {
		parts = append(parts, s.Arch)
	}
	if s.Virtualization != "" && s.Virtualization != "none" {
		parts = append(parts, s.Virtualization)
	}
	if len(parts) == 0 {
		return "Workspace"
	}
	return strings.Join(parts, " · ")
}

func summarize(s Snapshot) string {
	return fmt.Sprintf("%s %s (%s) · %s · %d cores · %s · %s",
		firstNonEmpty(s.Distro, s.OS),
		s.DistroVersion,
		s.Arch,
		firstNonEmpty(s.Virtualization, string(s.Type)),
		s.CPUCores,
		s.MemoryHuman,
		s.PrimaryIP,
	)
}

func resolveMachineID(productUUID string) string {
	if v := strings.TrimSpace(viper.GetString("CONTAINERWS_MACHINE_ID")); v != "" {
		return v
	}
	if v := strings.TrimSpace(viper.GetString("CONTAINER_ID")); v != "" {
		return v
	}
	// Prefer durable id on the /config volume (survives container recreate).
	if id := strings.TrimSpace(readFirstLine(stableMachineIDPath())); id != "" {
		return id
	}
	if id := strings.TrimSpace(readFirstLine("/etc/machine-id")); id != "" {
		return id
	}
	if id := strings.TrimSpace(readFirstLine("/var/lib/dbus/machine-id")); id != "" {
		return id
	}
	if productUUID != "" && productUUID != "None" {
		return productUUID
	}
	hostname, _ := os.Hostname()
	return "host-" + hostname + "-" + runtime.GOARCH
}

func detectVirtualization(sysVendor, productName string) (virt, runtimeName, cloud string, isCtr, isVM bool, ctype models.ContainerType) {
	ctype = models.ContainerTypeUnknown
	virt = "none"

	if os.Getenv("WSL_DISTRO_NAME") != "" || strings.Contains(strings.ToLower(readFirstLine("/proc/version")), "microsoft") {
		return "wsl", "", "", true, true, models.ContainerTypeWSL
	}

	if _, err := os.Stat("/.dockerenv"); err == nil {
		isCtr = true
		runtimeName = "docker"
		virt = "docker"
		ctype = models.ContainerTypeContainer
	}
	if _, err := os.Stat("/run/.containerenv"); err == nil {
		isCtr = true
		if runtimeName == "" {
			runtimeName = "podman"
		}
		virt = firstNonEmpty(virt, "podman")
		ctype = models.ContainerTypeContainer
	}
	if v := os.Getenv("container"); v != "" {
		isCtr = true
		if runtimeName == "" {
			runtimeName = v
		}
		virt = firstNonEmpty(virt, v)
		ctype = models.ContainerTypeContainer
	}
	if cg := readFileLimit("/proc/1/cgroup", 4096); strings.Contains(cg, "docker") || strings.Contains(cg, "containerd") || strings.Contains(cg, "kubepods") {
		isCtr = true
		if strings.Contains(cg, "kubepods") {
			runtimeName = firstNonEmpty(runtimeName, "kubernetes")
			cloud = firstNonEmpty(cloud, detectCloud())
		}
		virt = firstNonEmpty(virt, "container")
		ctype = models.ContainerTypeContainer
	}

	vendor := strings.ToLower(sysVendor + " " + productName)
	switch {
	case strings.Contains(vendor, "google"):
		cloud = "gcp"
		isVM = true
		virt = firstNonEmpty(virt, "gce")
	case strings.Contains(vendor, "amazon") || strings.Contains(vendor, "ec2"):
		cloud = "aws"
		isVM = true
		virt = firstNonEmpty(virt, "ec2")
	case strings.Contains(vendor, "microsoft corporation") && !strings.Contains(vendor, "wsl"):
		cloud = firstNonEmpty(cloud, "azure")
		isVM = true
		virt = firstNonEmpty(virt, "hyper-v")
	case strings.Contains(vendor, "vmware"):
		isVM = true
		virt = firstNonEmpty(virt, "vmware")
	case strings.Contains(vendor, "qemu") || strings.Contains(vendor, "kvm"):
		isVM = true
		virt = firstNonEmpty(virt, "kvm")
	case strings.Contains(vendor, "xen"):
		isVM = true
		virt = firstNonEmpty(virt, "xen")
	case strings.Contains(vendor, "virtualbox"):
		isVM = true
		virt = firstNonEmpty(virt, "virtualbox")
	}

	if isCtr {
		ctype = models.ContainerTypeContainer
	} else if strings.Contains(strings.ToLower(os.Getenv("WSL_DISTRO_NAME")+readFirstLine("/proc/version")), "microsoft") {
		ctype = models.ContainerTypeWSL
	} else if isVM {
		ctype = models.ContainerTypeVM
	} else if virt == "none" {
		ctype = models.ContainerTypePhysical
	}

	if cloud == "" {
		cloud = detectCloud()
	}
	return virt, runtimeName, cloud, isCtr, isVM, ctype
}

func detectCloud() string {
	// Lightweight hints only (no network calls).
	if _, err := os.Stat("/sys/class/dmi/id/product_serial"); err == nil {
		serial := strings.ToLower(readDMI("product_serial"))
		if strings.HasPrefix(serial, "ec2") {
			return "aws"
		}
	}
	return ""
}

func collectAddresses() (ips, macs []string, primary, ifaceName string) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, nil, "", ""
	}
	seenIP := map[string]struct{}{}
	seenMAC := map[string]struct{}{}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagLoopback != 0 || iface.Flags&net.FlagUp == 0 {
			continue
		}
		if hw := iface.HardwareAddr.String(); hw != "" {
			if _, ok := seenMAC[hw]; !ok {
				seenMAC[hw] = struct{}{}
				macs = append(macs, hw)
			}
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, a := range addrs {
			var ip net.IP
			switch v := a.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			if ip == nil || ip.IsLoopback() {
				continue
			}
			// Prefer IPv4 for primary.
			s := ip.String()
			if _, ok := seenIP[s]; ok {
				continue
			}
			seenIP[s] = struct{}{}
			ips = append(ips, s)
			if primary == "" && ip.To4() != nil {
				primary = s
				ifaceName = iface.Name
			}
		}
	}
	if primary == "" && len(ips) > 0 {
		primary = ips[0]
	}
	return ips, macs, primary, ifaceName
}

func readOSRelease() (id, name, ver string) {
	f, err := os.Open("/etc/os-release")
	if err != nil {
		return "", "", ""
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	vals := map[string]string{}
	for sc.Scan() {
		line := sc.Text()
		if i := strings.IndexByte(line, '='); i > 0 {
			k := line[:i]
			v := strings.Trim(line[i+1:], `"`)
			vals[k] = v
		}
	}
	return vals["ID"], firstNonEmpty(vals["NAME"], vals["PRETTY_NAME"]), firstNonEmpty(vals["VERSION_ID"], vals["VERSION"])
}

func readCPUInfo() (model string, cores int) {
	f, err := os.Open("/proc/cpuinfo")
	if err != nil {
		return "", runtime.NumCPU()
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		if strings.HasPrefix(line, "model name") || strings.HasPrefix(line, "Hardware") || strings.HasPrefix(line, "Processor") {
			if i := strings.Index(line, ":"); i >= 0 && model == "" {
				model = strings.TrimSpace(line[i+1:])
			}
		}
		if strings.HasPrefix(line, "processor") {
			cores++
		}
	}
	if cores == 0 {
		cores = runtime.NumCPU()
	}
	return model, cores
}

func readMemTotal() int64 {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return 0
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		if strings.HasPrefix(line, "MemTotal:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				var kb int64
				fmt.Sscanf(fields[1], "%d", &kb)
				return kb * 1024
			}
		}
	}
	return 0
}

func readDMI(key string) string {
	return strings.TrimSpace(readFirstLine("/sys/class/dmi/id/" + key))
}

func readFirstLine(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	line, _, _ := strings.Cut(string(b), "\n")
	return strings.TrimSpace(line)
}

func readFileLimit(path string, n int) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	buf := make([]byte, n)
	nr, _ := f.Read(buf)
	return string(buf[:nr])
}

func humanBytes(b int64) string {
	if b <= 0 {
		return ""
	}
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(b)/float64(div), "KMGTPE"[exp])
}

func archLabel(arch string) string {
	switch arch {
	case "amd64":
		return "x86_64"
	case "arm64":
		return "aarch64"
	default:
		return arch
	}
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func firstWord(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, ' '); i > 0 {
		return s[:i]
	}
	return s
}
