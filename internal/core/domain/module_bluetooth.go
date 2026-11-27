package domain

import (
	"net"
	"regexp"
	"strings"
)

// bluetoothVersionPattern is a Bluetooth Core Specification version such as
// "4.2" or "5.3".
var bluetoothVersionPattern = regexp.MustCompile(`^[1-9][0-9]?\.[0-9]$`)

// Bluetooth is a device's Bluetooth radio: the payload of a ModuleBluetooth
// module. Every field may be empty.
type Bluetooth struct {
	// Name is the name the device advertises.
	Name string
	// Address is the Bluetooth device address (BD_ADDR), an EUI-48 in lower
	// case and colon-separated.
	Address string
	// Version is the supported Core Specification version, e.g. "5.3".
	Version string
}

// String describes b in one line, for the audit log.
func (b Bluetooth) String() string {
	return strings.Join([]string{b.Name, b.Address, b.Version}, " / ")
}

// BluetoothParams is unvalidated input for a Bluetooth module.
type BluetoothParams struct {
	Name    string
	Address string
	Version string
}

func checkBluetooth(f fieldErrors, field string, p BluetoothParams) Bluetooth {
	version := strings.TrimSpace(p.Version)
	if version != "" && !bluetoothVersionPattern.MatchString(version) {
		f.add(field+".version", FieldInvalid)
	}

	return Bluetooth{
		Name:    checkOptionalText(f, field+".name", p.Name, maxShortTextRunes),
		Address: checkOptionalMAC(f, field+".address", p.Address),
		Version: version,
	}
}

// checkOptionalMAC is checkMAC for an address that may be empty.
func checkOptionalMAC(f fieldErrors, field, s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}

	mac, err := net.ParseMAC(s)
	if err != nil || len(mac) != 6 {
		f.add(field, FieldInvalid)
		return s
	}

	return mac.String()
}
