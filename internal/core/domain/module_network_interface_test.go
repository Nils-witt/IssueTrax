package domain_test

import (
	"net/netip"
	"strings"
	"testing"

	"github.com/google/uuid"

	"nilswitt.dev/issuetrax/internal/core/domain"
)

func dhcpInterface() domain.NetworkInterfaceParams {
	return domain.NetworkInterfaceParams{
		Name: "eth0", MAC: "AA-BB-CC-00-11-22", Type: domain.NetworkInterfaceWired,
		IPv4: domain.IPConfigParams{Mode: domain.IPModeDHCP},
		IPv6: domain.IPConfigParams{Mode: domain.IPModeOff},
	}
}

// itemWith builds a valid item whose only module is m.
func itemWith(m domain.ModuleParams) (domain.StorageItem, error) {
	p := validItem()
	p.Modules = []domain.ModuleParams{m}

	return domain.NewStorageItem(p)
}

func TestNetworkInterfaceModule(t *testing.T) {
	t.Parallel()

	item, err := itemWith(domain.ModuleParams{Kind: domain.ModuleNetworkInterface, NetworkInterface: &domain.NetworkInterfaceParams{
		Name: " eth0 ", MAC: "AA-BB-CC-00-11-22", Type: domain.NetworkInterfaceWired,
		NativeVLAN: new(1), TaggedVLANs: []int{4094, 20},
		IPv4: domain.IPConfigParams{
			Mode: domain.IPModeStatic, Address: " 192.168.1.10 ", PrefixLength: new(24),
			Gateway: "192.168.1.1", DNSServers: []string{"192.168.1.1", "9.9.9.9"},
		},
		IPv6: domain.IPConfigParams{
			Mode: domain.IPModeStatic, Address: "2001:db8::10", PrefixLength: new(64), Gateway: "fe80::1",
			DNSServers: []string{"2001:db8::53"},
		},
	}})
	if err != nil {
		t.Fatal(err)
	}

	want := domain.NetworkInterface{
		Name: "eth0", MAC: "aa:bb:cc:00:11:22", Type: domain.NetworkInterfaceWired, NativeVLAN: 1, TaggedVLANs: []int{4094, 20},
		IPv4: domain.IPConfig{
			Mode: domain.IPModeStatic, Address: netip.MustParsePrefix("192.168.1.10/24"), Gateway: netip.MustParseAddr("192.168.1.1"),
			DNSServers: []netip.Addr{netip.MustParseAddr("192.168.1.1"), netip.MustParseAddr("9.9.9.9")},
		},
		IPv6: domain.IPConfig{
			Mode: domain.IPModeStatic, Address: netip.MustParsePrefix("2001:db8::10/64"), Gateway: netip.MustParseAddr("fe80::1"),
			DNSServers: []netip.Addr{netip.MustParseAddr("2001:db8::53")},
		},
	}

	if got := item.Modules[0].NetworkInterface; !got.Equal(want) {
		t.Fatalf("interface = %+v, want %+v", got, want)
	}
}

// interfaceValidationCases each change a valid interface and list the
// resulting field errors (below "modules[0].networkInterface").
var interfaceValidationCases = []struct {
	name   string
	change func(p *domain.NetworkInterfaceParams)
	want   map[string]string
}{
	{
		name:   "missing fields",
		change: func(p *domain.NetworkInterfaceParams) { *p = domain.NetworkInterfaceParams{} },
		want: map[string]string{
			"name": domain.FieldRequired, "mac": domain.FieldRequired, "type": domain.FieldRequired,
			"ipv4.mode": domain.FieldRequired, "ipv6.mode": domain.FieldRequired,
		},
	},
	{
		name: "bad mac and type",
		change: func(p *domain.NetworkInterfaceParams) {
			p.MAC = "00:00:00:00:fe:80:00:00:00:00:00:00:00:00:00:01"
			p.Type = "fiber"
		},
		want: map[string]string{"mac": domain.FieldInvalid, "type": domain.FieldInvalid},
	},
	{
		name:   "ssid on wired",
		change: func(p *domain.NetworkInterfaceParams) { p.SSID = "Office" },
		want:   map[string]string{"ssid": domain.FieldInvalid},
	},
	{
		name: "ssid too long",
		change: func(p *domain.NetworkInterfaceParams) {
			p.Type = domain.NetworkInterfaceWireless
			p.SSID = strings.Repeat("a", 33)
		},
		want: map[string]string{"ssid": domain.FieldTooLong},
	},
	{
		name: "vlans",
		change: func(p *domain.NetworkInterfaceParams) {
			p.NativeVLAN = new(5)
			p.TaggedVLANs = []int{6, 0, 6, 5}
		},
		want: map[string]string{
			"taggedVlans[1]": domain.FieldOutOfRange, "taggedVlans[2]": domain.FieldInvalid, "taggedVlans[3]": domain.FieldInvalid,
		},
	},
	{
		name:   "native vlan out of range",
		change: func(p *domain.NetworkInterfaceParams) { p.NativeVLAN = new(4095) },
		want:   map[string]string{"nativeVlan": domain.FieldOutOfRange},
	},
	{
		name:   "slaac on ipv4",
		change: func(p *domain.NetworkInterfaceParams) { p.IPv4.Mode = domain.IPModeSLAAC },
		want:   map[string]string{"ipv4.mode": domain.FieldInvalid},
	},
	{
		name: "static settings outside static mode",
		change: func(p *domain.NetworkInterfaceParams) {
			p.IPv4 = domain.IPConfigParams{
				Mode: domain.IPModeDHCP, Address: "10.0.0.2", PrefixLength: new(24), Gateway: "10.0.0.1", DNSServers: []string{"10.0.0.1"},
			}
		},
		want: map[string]string{
			"ipv4.address": domain.FieldInvalid, "ipv4.prefixLength": domain.FieldInvalid,
			"ipv4.gateway": domain.FieldInvalid, "ipv4.dnsServers": domain.FieldInvalid,
		},
	},
	{
		name:   "static needs address and prefix",
		change: func(p *domain.NetworkInterfaceParams) { p.IPv4 = domain.IPConfigParams{Mode: domain.IPModeStatic} },
		want:   map[string]string{"ipv4.address": domain.FieldRequired, "ipv4.prefixLength": domain.FieldRequired},
	},
	{
		name: "wrong families",
		change: func(p *domain.NetworkInterfaceParams) {
			p.IPv4 = domain.IPConfigParams{
				Mode: domain.IPModeStatic, Address: "2001:db8::1", PrefixLength: new(33), DNSServers: []string{"::1"},
			}
			p.IPv6 = domain.IPConfigParams{
				Mode: domain.IPModeStatic, Address: "::ffff:10.0.0.1", PrefixLength: new(64), Gateway: "10.0.0.1",
			}
		},
		want: map[string]string{
			"ipv4.address": domain.FieldInvalid, "ipv4.prefixLength": domain.FieldOutOfRange,
			"ipv4.dnsServers[0]": domain.FieldInvalid, "ipv6.address": domain.FieldInvalid, "ipv6.gateway": domain.FieldInvalid,
		},
	},
	{
		name: "gateway outside subnet",
		change: func(p *domain.NetworkInterfaceParams) {
			p.IPv4 = domain.IPConfigParams{Mode: domain.IPModeStatic, Address: "10.0.0.2", PrefixLength: new(24), Gateway: "10.0.1.1"}
			p.IPv6 = domain.IPConfigParams{Mode: domain.IPModeStatic, Address: "2001:db8::2", PrefixLength: new(64), Gateway: "2001:db9::1"}
		},
		want: map[string]string{"ipv4.gateway": domain.FieldInvalid, "ipv6.gateway": domain.FieldInvalid},
	},
	{
		name: "duplicate dns and zone",
		change: func(p *domain.NetworkInterfaceParams) {
			p.IPv6 = domain.IPConfigParams{
				Mode: domain.IPModeStatic, Address: "fe80::2%eth0", PrefixLength: new(64),
				DNSServers: []string{"2001:db8::53", "2001:db8::53"},
			}
		},
		want: map[string]string{"ipv6.address": domain.FieldInvalid, "ipv6.dnsServers[1]": domain.FieldInvalid},
	},
}

func TestNetworkInterfaceValidation(t *testing.T) {
	t.Parallel()

	for _, tt := range interfaceValidationCases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			p := dhcpInterface()
			tt.change(&p)

			want := make(map[string]string, len(tt.want))
			for k, v := range tt.want {
				want["modules[0].networkInterface."+k] = v
			}

			_, err := itemWith(domain.ModuleParams{Kind: domain.ModuleNetworkInterface, NetworkInterface: &p})
			wantFields(t, err, want)
		})
	}
}

func TestNetworkHostAndHardwareModules(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		module domain.ModuleParams
		want   map[string]string
	}{
		{
			name:   "valid host",
			module: domain.ModuleParams{Kind: domain.ModuleNetworkHost, NetworkHost: &domain.NetworkHostParams{Role: domain.NetworkRoleRouter, Hostname: " gw.example.org "}},
		},
		{
			name:   "unknown role",
			module: domain.ModuleParams{Kind: domain.ModuleNetworkHost, NetworkHost: &domain.NetworkHostParams{Role: "firewall"}},
			want:   map[string]string{"modules[0].networkHost.role": domain.FieldInvalid},
		},
		{
			name: "hostname too long",
			module: domain.ModuleParams{Kind: domain.ModuleNetworkHost, NetworkHost: &domain.NetworkHostParams{
				Role: domain.NetworkRoleClient, Hostname: strings.Repeat("a.", 127) + "ab",
			}},
			want: map[string]string{"modules[0].networkHost.hostname": domain.FieldTooLong},
		},
		{
			name:   "serial too long",
			module: domain.ModuleParams{Kind: domain.ModuleHardware, Hardware: &domain.HardwareParams{SerialNumber: strings.Repeat("a", 101)}},
			want:   map[string]string{"modules[0].hardware.serialNumber": domain.FieldTooLong},
		},
		{
			name: "model without manufacturer",
			module: domain.ModuleParams{Kind: domain.ModuleHardware, Hardware: &domain.HardwareParams{
				ModelID: new(domain.HardwareModelID(uuid.New())),
			}},
			want: map[string]string{"modules[0].hardware.manufacturerId": domain.FieldRequired},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			item, err := itemWith(tt.module)
			wantFields(t, err, tt.want)

			if err == nil && item.Modules[0].NetworkHost.Hostname != "gw.example.org" {
				t.Fatalf("host = %+v", item.Modules[0].NetworkHost)
			}
		})
	}
}

func TestNetworkInterfaceString(t *testing.T) {
	t.Parallel()

	n := domain.NetworkInterface{
		Name: "wlan0", MAC: "02:00:00:00:00:01", Type: domain.NetworkInterfaceWireless, SSID: "Office",
		NativeVLAN: 10, TaggedVLANs: []int{20, 30},
		IPv4: domain.IPConfig{
			Mode: domain.IPModeStatic, Address: netip.MustParsePrefix("10.0.0.2/24"), Gateway: netip.MustParseAddr("10.0.0.1"),
			DNSServers: []netip.Addr{netip.MustParseAddr("10.0.0.1"), netip.MustParseAddr("9.9.9.9")},
		},
		IPv6: domain.IPConfig{Mode: domain.IPModeSLAAC},
	}

	want := `wlan0 02:00:00:00:00:01 wireless ssid "Office" native vlan 10 tagged vlans 20,30; ` +
		`ipv4 static 10.0.0.2/24 gw 10.0.0.1 dns 10.0.0.1,9.9.9.9; ipv6 slaac`
	if got := n.String(); got != want {
		t.Fatalf("String() = %q, want %q", got, want)
	}
}
