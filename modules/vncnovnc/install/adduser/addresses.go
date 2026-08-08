package adduser

import (
	"net"
	"sort"
	"strings"
)

// BindAddressOption is a host interface address the VNC RFB can listen on.
type BindAddressOption struct {
	Address   string `json:"address"`
	Interface string `json:"interface"`
	Label     string `json:"label"`
	Localhost bool   `json:"localhost"`
	Family    string `json:"family"` // ipv4 | ipv6
}

// NormalizeBindAddress returns a usable listen address (defaults to loopback).
func NormalizeBindAddress(addr string) string {
	addr = strings.TrimSpace(addr)
	if addr == "" || strings.EqualFold(addr, "localhost") {
		return BindHost
	}
	if ip := net.ParseIP(addr); ip == nil {
		return BindHost
	}
	return addr
}

// IsLoopbackBind reports whether addr is loopback-only.
func IsLoopbackBind(addr string) bool {
	addr = NormalizeBindAddress(addr)
	ip := net.ParseIP(addr)
	return ip != nil && ip.IsLoopback()
}

// ListBindAddresses returns localhost plus up IPv4/IPv6 addresses on the machine.
func ListBindAddresses() []BindAddressOption {
	out := []BindAddressOption{{
		Address:   BindHost,
		Interface: "lo",
		Label:     "127.0.0.1 · localhost (panel proxy only)",
		Localhost: true,
		Family:    "ipv4",
	}}

	seen := map[string]struct{}{BindHost: {}}
	ifaces, err := net.Interfaces()
	if err != nil {
		return out
	}

	var extras []BindAddressOption
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 {
			continue
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
			if ip == nil || ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsMulticast() {
				continue
			}
			s := ip.String()
			if _, ok := seen[s]; ok {
				continue
			}
			seen[s] = struct{}{}
			family := "ipv4"
			if ip.To4() == nil {
				family = "ipv6"
			}
			scope := "LAN / public"
			if ip.IsPrivate() {
				scope = "private network"
			}
			extras = append(extras, BindAddressOption{
				Address:   s,
				Interface: iface.Name,
				Label:     s + " · " + iface.Name + " (" + scope + ")",
				Localhost: false,
				Family:    family,
			})
		}
	}

	sort.SliceStable(extras, func(i, j int) bool {
		if extras[i].Family != extras[j].Family {
			return extras[i].Family == "ipv4"
		}
		return extras[i].Address < extras[j].Address
	})
	return append(out, extras...)
}

// IsAddressAllowed reports whether addr is localhost or a current host address.
func IsAddressAllowed(addr string) bool {
	addr = NormalizeBindAddress(addr)
	for _, opt := range ListBindAddresses() {
		if opt.Address == addr {
			return true
		}
	}
	return false
}
