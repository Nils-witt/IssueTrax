package domain

import (
	"fmt"
	"net"
	"net/netip"
	"slices"
	"strconv"
	"strings"
)

// Network interface limits.
const (
	maxInterfaceNameRunes = 100
	// maxSSIDBytes is the 802.11 limit for a network name.
	maxSSIDBytes = 32
	// MinVLAN and MaxVLAN bound usable VLAN IDs (0 and 4095 are reserved).
	MinVLAN = 1
	MaxVLAN = 4094
)

// NetworkInterfaceType is the physical medium of a network interface.
type NetworkInterfaceType string

// Network interface types.
const (
	NetworkInterfaceWired    NetworkInterfaceType = "wired"
	NetworkInterfaceWireless NetworkInterfaceType = "wireless"
)

// Valid reports whether t is a known interface type.
func (t NetworkInterfaceType) Valid() bool {
	return t == NetworkInterfaceWired || t == NetworkInterfaceWireless
}

// IPMode is how an interface gets its address for one IP family.
type IPMode string

// IP modes. SLAAC only exists for IPv6.
const (
	IPModeDHCP   IPMode = "dhcp"
	IPModeSLAAC  IPMode = "slaac"
	IPModeStatic IPMode = "static"
	IPModeOff    IPMode = "off"
)

// IPConfig is an interface's configuration for one IP family. Address,
// Gateway and DNSServers are only set in IPModeStatic.
type IPConfig struct {
	Mode IPMode
	// Address is the interface address with its prefix length; the host
	// bits are kept. Invalid (zero) unless static.
	Address netip.Prefix
	// Gateway is invalid (zero) when there is none.
	Gateway    netip.Addr
	DNSServers []netip.Addr
}

// Equal reports whether c and o hold the same configuration.
func (c IPConfig) Equal(o IPConfig) bool {
	return c.Mode == o.Mode && c.Address == o.Address && c.Gateway == o.Gateway && slices.Equal(c.DNSServers, o.DNSServers)
}

// String describes c in one line, e.g. "static 10.0.0.2/24 gw 10.0.0.1 dns 10.0.0.1".
func (c IPConfig) String() string {
	var b strings.Builder

	b.WriteString(string(c.Mode))

	if c.Address.IsValid() {
		b.WriteString(" " + c.Address.String())
	}

	if c.Gateway.IsValid() {
		b.WriteString(" gw " + c.Gateway.String())
	}

	for i, d := range c.DNSServers {
		if i == 0 {
			b.WriteString(" dns ")
		} else {
			b.WriteString(",")
		}

		b.WriteString(d.String())
	}

	return b.String()
}

// NetworkInterface is one network interface: the payload of a
// ModuleNetworkInterface module.
type NetworkInterface struct {
	// Name labels the interface, e.g. "eth0".
	Name string
	// MAC is the EUI-48 address in lower case, colon-separated. MACs need
	// not be unique.
	MAC  string
	Type NetworkInterfaceType
	// SSID is the wireless network name; always empty for wired interfaces.
	SSID string
	// NativeVLAN is the untagged VLAN ID; 0 means none.
	NativeVLAN int
	// TaggedVLANs are distinct VLAN IDs, never including NativeVLAN.
	TaggedVLANs []int
	IPv4        IPConfig
	IPv6        IPConfig
}

// Equal reports whether n and o are the same interface configuration.
func (n NetworkInterface) Equal(o NetworkInterface) bool {
	return n.Name == o.Name && n.MAC == o.MAC && n.Type == o.Type && n.SSID == o.SSID && n.NativeVLAN == o.NativeVLAN &&
		slices.Equal(n.TaggedVLANs, o.TaggedVLANs) && n.IPv4.Equal(o.IPv4) && n.IPv6.Equal(o.IPv6)
}

// String describes n in one line, for the audit log.
func (n NetworkInterface) String() string {
	var b strings.Builder

	fmt.Fprintf(&b, "%s %s %s", n.Name, n.MAC, n.Type)

	if n.SSID != "" {
		fmt.Fprintf(&b, " ssid %q", n.SSID)
	}

	if n.NativeVLAN != 0 {
		fmt.Fprintf(&b, " native vlan %d", n.NativeVLAN)
	}

	if len(n.TaggedVLANs) > 0 {
		tagged := make([]string, 0, len(n.TaggedVLANs))
		for _, v := range n.TaggedVLANs {
			tagged = append(tagged, strconv.Itoa(v))
		}

		b.WriteString(" tagged vlans " + strings.Join(tagged, ","))
	}

	fmt.Fprintf(&b, "; ipv4 %s; ipv6 %s", n.IPv4, n.IPv6)

	return b.String()
}

// IPConfigParams is unvalidated input for one IP family of an interface.
// Address, PrefixLength, Gateway and DNSServers are only allowed when Mode
// is IPModeStatic; Address and PrefixLength are then required.
type IPConfigParams struct {
	Mode         IPMode
	Address      string
	PrefixLength *int
	Gateway      string
	DNSServers   []string
}

// NetworkInterfaceParams is unvalidated input for one interface.
type NetworkInterfaceParams struct {
	Name        string
	MAC         string
	Type        NetworkInterfaceType
	SSID        string
	NativeVLAN  *int
	TaggedVLANs []int
	IPv4        IPConfigParams
	IPv6        IPConfigParams
}

func checkInterface(f fieldErrors, field string, p NetworkInterfaceParams) NetworkInterface {
	n := NetworkInterface{
		Name: checkRequiredText(f, field+".name", p.Name, maxInterfaceNameRunes),
		MAC:  checkMAC(f, field+".mac", p.MAC),
		Type: p.Type,
		SSID: strings.TrimSpace(p.SSID),
	}

	switch {
	case p.Type == "":
		f.add(field+".type", FieldRequired)
	case !p.Type.Valid():
		f.add(field+".type", FieldInvalid)
	}

	switch {
	case n.SSID == "":
	case p.Type == NetworkInterfaceWired:
		f.add(field+".ssid", FieldInvalid)
	case len(n.SSID) > maxSSIDBytes:
		f.add(field+".ssid", FieldTooLong)
	}

	if p.NativeVLAN != nil {
		if validVLAN(*p.NativeVLAN) {
			n.NativeVLAN = *p.NativeVLAN
		} else {
			f.add(field+".nativeVlan", FieldOutOfRange)
		}
	}

	n.TaggedVLANs = checkTaggedVLANs(f, field+".taggedVlans", p.TaggedVLANs, n.NativeVLAN)
	n.IPv4 = checkIPConfig(f, field+".ipv4", p.IPv4, false)
	n.IPv6 = checkIPConfig(f, field+".ipv6", p.IPv6, true)

	return n
}

func validVLAN(v int) bool { return v >= MinVLAN && v <= MaxVLAN }

func checkTaggedVLANs(f fieldErrors, field string, vlans []int, native int) []int {
	out := make([]int, 0, len(vlans))

	for i, v := range vlans {
		at := fmt.Sprintf("%s[%d]", field, i)

		switch {
		case !validVLAN(v):
			f.add(at, FieldOutOfRange)
		case v == native || slices.Contains(out, v):
			f.add(at, FieldInvalid)
		default:
			out = append(out, v)
		}
	}

	return out
}

// checkMAC parses s as an EUI-48 MAC address in any notation net.ParseMAC
// knows and returns it lower case and colon-separated.
func checkMAC(f fieldErrors, field, s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		f.add(field, FieldRequired)
		return ""
	}

	mac, err := net.ParseMAC(s)
	if err != nil || len(mac) != 6 {
		f.add(field, FieldInvalid)
		return s
	}

	return mac.String()
}

func checkIPConfig(f fieldErrors, field string, p IPConfigParams, v6 bool) IPConfig {
	checkIPMode(f, field+".mode", p.Mode, v6)

	if p.Mode != IPModeStatic {
		checkNoStaticSettings(f, field, p)

		return IPConfig{Mode: p.Mode}
	}

	c := IPConfig{Mode: p.Mode, Address: checkStaticAddress(f, field, p, v6)}
	c.Gateway = checkGateway(f, field+".gateway", p.Gateway, c.Address, v6)
	c.DNSServers = checkDNSServers(f, field+".dnsServers", p.DNSServers, v6)

	return c
}

func checkIPMode(f fieldErrors, field string, mode IPMode, v6 bool) {
	switch mode {
	case "":
		f.add(field, FieldRequired)
	case IPModeDHCP, IPModeStatic, IPModeOff:
	case IPModeSLAAC:
		if !v6 {
			f.add(field, FieldInvalid)
		}
	default:
		f.add(field, FieldInvalid)
	}
}

// checkNoStaticSettings rejects static settings outside static mode.
func checkNoStaticSettings(f fieldErrors, field string, p IPConfigParams) {
	if strings.TrimSpace(p.Address) != "" {
		f.add(field+".address", FieldInvalid)
	}

	if p.PrefixLength != nil {
		f.add(field+".prefixLength", FieldInvalid)
	}

	if strings.TrimSpace(p.Gateway) != "" {
		f.add(field+".gateway", FieldInvalid)
	}

	if len(p.DNSServers) > 0 {
		f.add(field+".dnsServers", FieldInvalid)
	}
}

// checkStaticAddress returns the address with its prefix length; it is
// invalid (zero) when either is missing or wrong.
func checkStaticAddress(f fieldErrors, field string, p IPConfigParams, v6 bool) netip.Prefix {
	addr, addrOK := checkAddr(f, field+".address", p.Address, v6, true)

	bits := 32
	if v6 {
		bits = 128
	}

	switch {
	case p.PrefixLength == nil:
		f.add(field+".prefixLength", FieldRequired)
	case *p.PrefixLength < 0 || *p.PrefixLength > bits:
		f.add(field+".prefixLength", FieldOutOfRange)
	case addrOK:
		return netip.PrefixFrom(addr, *p.PrefixLength)
	}

	return netip.Prefix{}
}

// checkGateway parses an optional gateway, which must lie in subnet (or,
// for IPv6, be link-local). Without a valid subnet that error is already
// recorded, so only the address itself is checked.
func checkGateway(f fieldErrors, field, s string, subnet netip.Prefix, v6 bool) netip.Addr {
	gw, ok := checkAddr(f, field, s, v6, false)
	if !ok {
		return netip.Addr{}
	}

	inSubnet := !subnet.IsValid() || subnet.Masked().Contains(gw)
	if !inSubnet && (!v6 || !gw.IsLinkLocalUnicast()) {
		f.add(field, FieldInvalid)
	}

	return gw
}

func checkDNSServers(f fieldErrors, field string, servers []string, v6 bool) []netip.Addr {
	out := make([]netip.Addr, 0, len(servers))

	for i, s := range servers {
		at := fmt.Sprintf("%s[%d]", field, i)

		dns, ok := checkAddr(f, at, s, v6, true)

		switch {
		case !ok:
		case slices.Contains(out, dns):
			f.add(at, FieldInvalid)
		default:
			out = append(out, dns)
		}
	}

	return out
}

// checkAddr parses s as an address of the given family, without a zone.
// Empty s is required or absent (ok false, no error).
func checkAddr(f fieldErrors, field, s string, v6, required bool) (netip.Addr, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		if required {
			f.add(field, FieldRequired)
		}

		return netip.Addr{}, false
	}

	addr, err := netip.ParseAddr(s)
	if err != nil || addr.Zone() != "" || (v6 && (!addr.Is6() || addr.Is4In6())) || (!v6 && !addr.Is4()) {
		f.add(field, FieldInvalid)
		return netip.Addr{}, false
	}

	return addr, true
}

// NormalizeMAC returns s as a lower-case, colon-separated EUI-48 MAC
// address; ok is false when s is not one in any notation net.ParseMAC knows.
func NormalizeMAC(s string) (mac string, ok bool) {
	hw, err := net.ParseMAC(strings.TrimSpace(s))
	if err != nil || len(hw) != 6 {
		return "", false
	}

	return hw.String(), true
}
