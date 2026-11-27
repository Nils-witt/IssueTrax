package domain

import "strings"

// Host name limits.
const (
	maxHostnameBytes      = 253
	maxHostnameLabelBytes = 63
)

// NetworkRole is what a device does in the network.
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

func checkRole(f fieldErrors, field string, r NetworkRole) NetworkRole {
	switch {
	case r == "":
		f.add(field, FieldRequired)
	case !r.Valid():
		f.add(field, FieldInvalid)
	}

	return r
}

// checkHostname trims s and checks it is empty or an RFC 1123 host name:
// dot-separated labels of letters, digits and hyphens, not starting or
// ending with a hyphen.
func checkHostname(f fieldErrors, field, s string) string {
	s = strings.TrimSpace(s)

	switch {
	case s == "":
	case len(s) > maxHostnameBytes:
		f.add(field, FieldTooLong)
	case !validHostname(s):
		f.add(field, FieldInvalid)
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

// NetworkHost is a device's place in the network: the payload of a
// ModuleNetworkHost module. Hostname may be empty.
type NetworkHost struct {
	Role     NetworkRole
	Hostname string
}

// String describes h in one line, for the audit log.
func (h NetworkHost) String() string {
	if h.Hostname == "" {
		return string(h.Role)
	}

	return string(h.Role) + " " + h.Hostname
}

// NetworkHostParams is unvalidated input for a network host module.
type NetworkHostParams struct {
	Role     NetworkRole
	Hostname string
}

func checkNetworkHost(f fieldErrors, field string, p NetworkHostParams) NetworkHost {
	return NetworkHost{
		Role:     checkRole(f, field+".role", p.Role),
		Hostname: checkHostname(f, field+".hostname", p.Hostname),
	}
}
