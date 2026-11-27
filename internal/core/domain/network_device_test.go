package domain_test

import (
	"errors"
	"net/netip"
	"strings"
	"testing"
	"time"

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

func validDevice() domain.NewNetworkDeviceParams {
	return domain.NewNetworkDeviceParams{
		StorageID: domain.StorageID(uuid.New()), Name: "Router", Role: domain.NetworkRoleRouter,
		Primary: dhcpInterface(), Now: testNow,
	}
}

func TestNewNetworkDevice(t *testing.T) {
	t.Parallel()

	device, err := domain.NewNetworkDevice(domain.NewNetworkDeviceParams{
		StorageID: domain.StorageID(uuid.New()), Name: " Router ", Description: " rack ", Role: domain.NetworkRoleRouter,
		Hostname: " gw.example.org ", Manufacturer: " ACME ", Model: "R1", SerialNumber: "SN",
		Primary: domain.NetworkInterfaceParams{
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
		},
		Secondary: []domain.NetworkInterfaceParams{{
			Name: "wlan0", MAC: "02:00:00:00:00:01", Type: domain.NetworkInterfaceWireless, SSID: " Office ",
			IPv4: domain.IPConfigParams{Mode: domain.IPModeDHCP},
			IPv6: domain.IPConfigParams{Mode: domain.IPModeSLAAC},
		}},
		Now: testNow,
	})
	if err != nil {
		t.Fatalf("NewNetworkDevice: %v", err)
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

	if !device.Primary.Equal(want) {
		t.Fatalf("primary = %+v, want %+v", device.Primary, want)
	}

	if device.Name != "Router" || device.Description != "rack" || device.Hostname != "gw.example.org" ||
		device.Manufacturer != "ACME" || device.Kind != domain.StorageItemKindNetworkDevice ||
		len(device.Secondary) != 1 || device.Secondary[0].SSID != "Office" || device.Secondary[0].IPv6.Mode != domain.IPModeSLAAC {
		t.Fatalf("device = %+v", device)
	}
}

// networkDeviceValidationCases each change a valid device and list the
// resulting field errors.
var networkDeviceValidationCases = []struct {
	name   string
	change func(p *domain.NewNetworkDeviceParams)
	want   map[string]string
}{
	{
		name:   "missing role",
		change: func(p *domain.NewNetworkDeviceParams) { p.Role = "" },
		want:   map[string]string{"role": domain.FieldRequired},
	},
	{
		name:   "unknown role",
		change: func(p *domain.NewNetworkDeviceParams) { p.Role = "firewall" },
		want:   map[string]string{"role": domain.FieldInvalid},
	},
	{
		name:   "bad hostname",
		change: func(p *domain.NewNetworkDeviceParams) { p.Hostname = "-bad_host" },
		want:   map[string]string{"hostname": domain.FieldInvalid},
	},
	{
		name:   "hostname too long",
		change: func(p *domain.NewNetworkDeviceParams) { p.Hostname = strings.Repeat("a.", 127) + "ab" },
		want:   map[string]string{"hostname": domain.FieldTooLong},
	},
	{
		name:   "serial too long",
		change: func(p *domain.NewNetworkDeviceParams) { p.SerialNumber = strings.Repeat("a", 101) },
		want:   map[string]string{"serialNumber": domain.FieldTooLong},
	},
	{
		name: "missing interface fields",
		change: func(p *domain.NewNetworkDeviceParams) {
			p.Primary = domain.NetworkInterfaceParams{}
		},
		want: map[string]string{
			"primaryInterface.name": domain.FieldRequired, "primaryInterface.mac": domain.FieldRequired,
			"primaryInterface.type": domain.FieldRequired, "primaryInterface.ipv4.mode": domain.FieldRequired,
			"primaryInterface.ipv6.mode": domain.FieldRequired,
		},
	},
	{
		name: "bad mac and type",
		change: func(p *domain.NewNetworkDeviceParams) {
			p.Primary.MAC = "00:00:00:00:fe:80:00:00:00:00:00:00:00:00:00:01"
			p.Primary.Type = "fiber"
		},
		want: map[string]string{"primaryInterface.mac": domain.FieldInvalid, "primaryInterface.type": domain.FieldInvalid},
	},
	{
		name: "ssid on wired",
		change: func(p *domain.NewNetworkDeviceParams) {
			p.Primary.SSID = "Office"
		},
		want: map[string]string{"primaryInterface.ssid": domain.FieldInvalid},
	},
	{
		name: "ssid too long",
		change: func(p *domain.NewNetworkDeviceParams) {
			p.Primary.Type = domain.NetworkInterfaceWireless
			p.Primary.SSID = strings.Repeat("a", 33)
		},
		want: map[string]string{"primaryInterface.ssid": domain.FieldTooLong},
	},
	{
		name: "vlans",
		change: func(p *domain.NewNetworkDeviceParams) {
			p.Primary.NativeVLAN = new(4095)
			p.Secondary = []domain.NetworkInterfaceParams{dhcpInterface()}
			p.Secondary[0].NativeVLAN = new(5)
			p.Secondary[0].TaggedVLANs = []int{6, 0, 6, 5}
		},
		want: map[string]string{
			"primaryInterface.nativeVlan":           domain.FieldOutOfRange,
			"secondaryInterfaces[0].taggedVlans[1]": domain.FieldOutOfRange,
			"secondaryInterfaces[0].taggedVlans[2]": domain.FieldInvalid,
			"secondaryInterfaces[0].taggedVlans[3]": domain.FieldInvalid,
		},
	},
	{
		name: "slaac on ipv4",
		change: func(p *domain.NewNetworkDeviceParams) {
			p.Primary.IPv4.Mode = domain.IPModeSLAAC
		},
		want: map[string]string{"primaryInterface.ipv4.mode": domain.FieldInvalid},
	},
	{
		name: "static settings outside static mode",
		change: func(p *domain.NewNetworkDeviceParams) {
			p.Primary.IPv4 = domain.IPConfigParams{
				Mode: domain.IPModeDHCP, Address: "10.0.0.2", PrefixLength: new(24), Gateway: "10.0.0.1", DNSServers: []string{"10.0.0.1"},
			}
		},
		want: map[string]string{
			"primaryInterface.ipv4.address": domain.FieldInvalid, "primaryInterface.ipv4.prefixLength": domain.FieldInvalid,
			"primaryInterface.ipv4.gateway": domain.FieldInvalid, "primaryInterface.ipv4.dnsServers": domain.FieldInvalid,
		},
	},
	{
		name: "static needs address and prefix",
		change: func(p *domain.NewNetworkDeviceParams) {
			p.Primary.IPv4 = domain.IPConfigParams{Mode: domain.IPModeStatic}
		},
		want: map[string]string{
			"primaryInterface.ipv4.address": domain.FieldRequired, "primaryInterface.ipv4.prefixLength": domain.FieldRequired,
		},
	},
	{
		name: "wrong families",
		change: func(p *domain.NewNetworkDeviceParams) {
			p.Primary.IPv4 = domain.IPConfigParams{
				Mode: domain.IPModeStatic, Address: "2001:db8::1", PrefixLength: new(33), DNSServers: []string{"::1"},
			}
			p.Primary.IPv6 = domain.IPConfigParams{
				Mode: domain.IPModeStatic, Address: "::ffff:10.0.0.1", PrefixLength: new(64), Gateway: "10.0.0.1",
			}
		},
		want: map[string]string{
			"primaryInterface.ipv4.address": domain.FieldInvalid, "primaryInterface.ipv4.prefixLength": domain.FieldOutOfRange,
			"primaryInterface.ipv4.dnsServers[0]": domain.FieldInvalid,
			"primaryInterface.ipv6.address":       domain.FieldInvalid, "primaryInterface.ipv6.gateway": domain.FieldInvalid,
		},
	},
	{
		name: "gateway outside subnet",
		change: func(p *domain.NewNetworkDeviceParams) {
			p.Primary.IPv4 = domain.IPConfigParams{Mode: domain.IPModeStatic, Address: "10.0.0.2", PrefixLength: new(24), Gateway: "10.0.1.1"}
			p.Primary.IPv6 = domain.IPConfigParams{Mode: domain.IPModeStatic, Address: "2001:db8::2", PrefixLength: new(64), Gateway: "2001:db9::1"}
		},
		want: map[string]string{"primaryInterface.ipv4.gateway": domain.FieldInvalid, "primaryInterface.ipv6.gateway": domain.FieldInvalid},
	},
	{
		name: "duplicate dns and zone",
		change: func(p *domain.NewNetworkDeviceParams) {
			p.Primary.IPv6 = domain.IPConfigParams{
				Mode: domain.IPModeStatic, Address: "fe80::2%eth0", PrefixLength: new(64),
				DNSServers: []string{"2001:db8::53", "2001:db8::53"},
			}
		},
		want: map[string]string{"primaryInterface.ipv6.address": domain.FieldInvalid, "primaryInterface.ipv6.dnsServers[1]": domain.FieldInvalid},
	},
}

func TestNewNetworkDeviceValidation(t *testing.T) {
	t.Parallel()

	for _, tt := range networkDeviceValidationCases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			p := validDevice()
			tt.change(&p)

			_, err := domain.NewNetworkDevice(p)
			wantFields(t, err, tt.want)
		})
	}
}

// deviceWithSecondary is a valid device with a wired primary (eth0) and a
// wireless secondary (wlan0) interface, plus the params it was made from.
func deviceWithSecondary(t *testing.T) (domain.NetworkDevice, domain.NewNetworkDeviceParams) {
	t.Helper()

	p := validDevice()
	p.Secondary = []domain.NetworkInterfaceParams{{
		Name: "wlan0", MAC: "02:00:00:00:00:01", Type: domain.NetworkInterfaceWireless,
		IPv4: domain.IPConfigParams{Mode: domain.IPModeDHCP}, IPv6: domain.IPConfigParams{Mode: domain.IPModeSLAAC},
	}}

	device, err := domain.NewNetworkDevice(p)
	if err != nil {
		t.Fatal(err)
	}

	return device, p
}

func TestNetworkDeviceApplySwapsPrimary(t *testing.T) {
	t.Parallel()

	device, p := deviceWithSecondary(t)
	later := testNow.Add(time.Hour)

	// Unchanged values keep UpdatedAt.
	same := device
	if err := same.Apply(domain.NetworkDeviceChanges{Name: new("Router"), Primary: &p.Primary, Secondary: &p.Secondary}, later); err != nil ||
		!same.UpdatedAt.Equal(testNow) {
		t.Fatalf("Apply(no-op) = %v, updatedAt %v", err, same.UpdatedAt)
	}

	// Swapping makes the secondary interface primary.
	swapped := []domain.NetworkInterfaceParams{p.Primary}
	if err := device.Apply(domain.NetworkDeviceChanges{
		Role: new(domain.NetworkRoleSwitch), Primary: &p.Secondary[0], Secondary: &swapped,
	}, later); err != nil {
		t.Fatal(err)
	}

	if device.Primary.Name != "wlan0" || len(device.Secondary) != 1 || device.Secondary[0].Name != "eth0" ||
		device.Role != domain.NetworkRoleSwitch || !device.UpdatedAt.Equal(later) {
		t.Fatalf("after swap = %+v", device)
	}
}

func TestNetworkDeviceApplyRejects(t *testing.T) {
	t.Parallel()

	device, _ := deviceWithSecondary(t)
	later := testNow.Add(time.Hour)

	// Invalid changes leave the device unchanged.
	before := device
	wantFields(t, device.Apply(domain.NetworkDeviceChanges{Name: new("x"), Role: new(domain.NetworkRole("hub"))}, later),
		map[string]string{"role": domain.FieldInvalid})

	if device.Name != before.Name || device.Role != before.Role {
		t.Fatalf("device changed on error: %+v", device)
	}

	// Deleted devices cannot change.
	if err := device.Delete(later); err != nil {
		t.Fatal(err)
	}

	if err := device.Apply(domain.NetworkDeviceChanges{Name: new("y")}, later); !errors.Is(err, domain.ErrStorageItemDeleted) {
		t.Fatalf("Apply(deleted) = %v", err)
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
