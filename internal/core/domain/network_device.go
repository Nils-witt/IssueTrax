package domain

import (
	"fmt"
	"net"
	"net/netip"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

// Network device limits.
const (
	maxDeviceTextRunes    = 100
	maxHostnameBytes      = 253
	maxHostnameLabelBytes = 63
	maxInterfaceNameRunes = 100
	// maxSSIDBytes is the 802.11 limit for a network name.
	maxSSIDBytes = 32
	// MinVLAN and MaxVLAN bound usable VLAN IDs (0 and 4095 are reserved).
	MinVLAN = 1
	MaxVLAN = 4094
)

// NetworkRole is what a network device does in the network.
type NetworkRole string

// Network roles.
const (
	NetworkRoleRouter NetworkRole = "router"
	NetworkRoleSwitch NetworkRole = "switch"
	NetworkRoleClient NetworkRole = "client"
)

// Valid reports whether r is a known role.
func (r NetworkRole) Valid() bool {
	return r == NetworkRoleRouter || r == NetworkRoleSwitch || r == NetworkRoleClient
}

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

// NetworkInterface is one network interface of a NetworkDevice.
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

// NetworkDevice is a storage item of kind StorageItemKindNetworkDevice: a
// device with exactly one primary and any number of secondary interfaces.
type NetworkDevice struct {
	StorageItem
	Role NetworkRole
	// Hostname, Manufacturer, Model and SerialNumber may be empty.
	Hostname     string
	Manufacturer string
	Model        string
	SerialNumber string
	Primary      NetworkInterface
	Secondary    []NetworkInterface
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

// NewNetworkDeviceParams are the inputs for NewNetworkDevice.
type NewNetworkDeviceParams struct {
	ID           StorageItemID
	StorageID    StorageID
	Name         string
	Description  string
	Role         NetworkRole
	Hostname     string
	Manufacturer string
	Model        string
	SerialNumber string
	Primary      NetworkInterfaceParams
	Secondary    []NetworkInterfaceParams
	Now          time.Time
}

// NewNetworkDevice validates p and builds a NetworkDevice. It does not
// check that the storage exists; that needs storage. Field errors use the
// API field names, e.g. "secondaryInterfaces[1].ipv4.gateway".
func NewNetworkDevice(p NewNetworkDeviceParams) (NetworkDevice, error) {
	f := fieldErrors{}

	device := NetworkDevice{
		ID:           p.ID,
		StorageID:    p.StorageID,
		Kind:         StorageItemKindNetworkDevice,
		Name:         checkRequiredText(f, "name", p.Name, maxProjectNameRunes),
		Description:  checkDescription(f, "description", p.Description),
		CreatedAt:    p.Now,
		UpdatedAt:    p.Now,
		Role:         checkRole(f, p.Role),
		Hostname:     checkHostname(f, p.Hostname),
		Manufacturer: checkOptionalText(f, "manufacturer", p.Manufacturer, maxDeviceTextRunes),
		Model:        checkOptionalText(f, "model", p.Model, maxDeviceTextRunes),
		SerialNumber: checkOptionalText(f, "serialNumber", p.SerialNumber, maxDeviceTextRunes),
		Primary:      checkInterface(f, "primaryInterface", p.Primary),
		Secondary:    checkInterfaces(f, p.Secondary),
	}

	if err := f.err(); err != nil {
		return NetworkDevice{}, err
	}

	return device, nil
}

// NetworkDeviceChanges lists the mutable fields; nil means unchanged.
// StorageID moves the device. Primary and Secondary each replace what is
// stored, so swapping them makes a secondary interface primary.
type NetworkDeviceChanges struct {
	Name         *string
	Description  *string
	StorageID    *StorageID
	Role         *NetworkRole
	Hostname     *string
	Manufacturer *string
	Model        *string
	SerialNumber *string
	Primary      *NetworkInterfaceParams
	Secondary    *[]NetworkInterfaceParams
}

// Apply validates and applies c. On error d is left unchanged. A deleted
// device fails with ErrStorageItemDeleted.
func (d *NetworkDevice) Apply(c NetworkDeviceChanges, now time.Time) error {
	if d.IsDeleted() {
		return ErrStorageItemDeleted
	}

	f := fieldErrors{}
	next := *d

	if c.Name != nil {
		next.Name = checkRequiredText(f, "name", *c.Name, maxProjectNameRunes)
	}

	if c.Description != nil {
		next.Description = checkDescription(f, "description", *c.Description)
	}

	if c.StorageID != nil {
		next.StorageID = *c.StorageID
	}

	next.applyDetails(f, c)

	if c.Primary != nil {
		next.Primary = checkInterface(f, "primaryInterface", *c.Primary)
	}

	if c.Secondary != nil {
		next.Secondary = checkInterfaces(f, *c.Secondary)
	}

	if err := f.err(); err != nil {
		return err
	}

	if !next.sameContent(*d) {
		next.UpdatedAt = now
	}

	*d = next

	return nil
}

// applyDetails applies the role and identification fields of c to d.
func (d *NetworkDevice) applyDetails(f fieldErrors, c NetworkDeviceChanges) {
	if c.Role != nil {
		d.Role = checkRole(f, *c.Role)
	}

	if c.Hostname != nil {
		d.Hostname = checkHostname(f, *c.Hostname)
	}

	if c.Manufacturer != nil {
		d.Manufacturer = checkOptionalText(f, "manufacturer", *c.Manufacturer, maxDeviceTextRunes)
	}

	if c.Model != nil {
		d.Model = checkOptionalText(f, "model", *c.Model, maxDeviceTextRunes)
	}

	if c.SerialNumber != nil {
		d.SerialNumber = checkOptionalText(f, "serialNumber", *c.SerialNumber, maxDeviceTextRunes)
	}
}

// sameContent reports whether d and o have the same mutable fields.
func (d NetworkDevice) sameContent(o NetworkDevice) bool {
	return d.Name == o.Name && d.Description == o.Description && d.StorageID == o.StorageID && d.Role == o.Role &&
		d.Hostname == o.Hostname && d.Manufacturer == o.Manufacturer && d.Model == o.Model &&
		d.SerialNumber == o.SerialNumber && d.Primary.Equal(o.Primary) &&
		slices.EqualFunc(d.Secondary, o.Secondary, NetworkInterface.Equal)
}

func checkRole(f fieldErrors, r NetworkRole) NetworkRole {
	switch {
	case r == "":
		f.add("role", FieldRequired)
	case !r.Valid():
		f.add("role", FieldInvalid)
	}

	return r
}

// checkOptionalText trims s and records too_long for field.
func checkOptionalText(f fieldErrors, field, s string, maxRunes int) string {
	s = strings.TrimSpace(s)
	if utf8.RuneCountInString(s) > maxRunes {
		f.add(field, FieldTooLong)
	}

	return s
}

// checkHostname trims s and checks it is empty or an RFC 1123 host name:
// dot-separated labels of letters, digits and hyphens, not starting or
// ending with a hyphen.
func checkHostname(f fieldErrors, s string) string {
	s = strings.TrimSpace(s)

	switch {
	case s == "":
	case len(s) > maxHostnameBytes:
		f.add("hostname", FieldTooLong)
	case !validHostname(s):
		f.add("hostname", FieldInvalid)
	}

	return s
}

func validHostname(s string) bool {
	for label := range strings.SplitSeq(s, ".") {
		if !validHostnameLabel(label) {
			return false
		}
	}

	return true
}

func validHostnameLabel(label string) bool {
	if label == "" || len(label) > maxHostnameLabelBytes || label[0] == '-' || label[len(label)-1] == '-' {
		return false
	}

	for _, c := range []byte(label) {
		letterOrDigit := c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9'
		if !letterOrDigit && c != '-' {
			return false
		}
	}

	return true
}

func checkInterfaces(f fieldErrors, ps []NetworkInterfaceParams) []NetworkInterface {
	out := make([]NetworkInterface, 0, len(ps))
	for i, p := range ps {
		out = append(out, checkInterface(f, fmt.Sprintf("secondaryInterfaces[%d]", i), p))
	}

	return out
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
