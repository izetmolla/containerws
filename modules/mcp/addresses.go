package mcp

import (
	"net"
	"sort"
	"strings"
)

// BindAddressOption is a listen address choice for the standalone MCP listener.
type BindAddressOption struct {
	Address   string `json:"address"`
	Interface string `json:"interface,omitempty"`
	Label     string `json:"label"`
	Family    string `json:"family,omitempty"` // ipv4 | ipv6 | any
}

// ListBindAddresses returns 0.0.0.0, localhost, and up IPv4/IPv6 host addresses.
func ListBindAddresses() []BindAddressOption {
	out := []BindAddressOption{
		{
			Address:   "0.0.0.0",
			Interface: "*",
			Label:     "0.0.0.0 · all interfaces",
			Family:    "any",
		},
		{
			Address:   "127.0.0.1",
			Interface: "lo",
			Label:     "127.0.0.1 · localhost",
			Family:    "ipv4",
		},
	}
	seen := map[string]struct{}{
		"0.0.0.0":   {},
		"127.0.0.1": {},
	}

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

// EnsureBindAddressOption adds addr to the option list when it is missing
// (e.g. a previously saved IP that is temporarily down).
func EnsureBindAddressOption(opts []BindAddressOption, addr string) []BindAddressOption {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return opts
	}
	for _, o := range opts {
		if o.Address == addr {
			return opts
		}
	}
	return append(opts, BindAddressOption{
		Address: addr,
		Label:   addr + " · saved",
		Family:  "ipv4",
	})
}
