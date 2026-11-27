package domain

import (
	"net/netip"
	"regexp"
	"strconv"
	"strings"
)

// maxPort is the largest TCP/UDP port number.
const maxPort = 65535

// sipExtensionPattern is a SIP extension or phone number: an optional "+"
// and 1 to 15 digits (E.164 allows at most 15), after separators are
// removed.
var sipExtensionPattern = regexp.MustCompile(`^\+?[0-9]{1,15}$`)

// SIPTransport is the transport protocol a SIP client registers with.
type SIPTransport string

// SIP transports. The empty transport means unknown or the client default.
const (
	SIPTransportUDP SIPTransport = "udp"
	SIPTransportTCP SIPTransport = "tcp"
	SIPTransportTLS SIPTransport = "tls"
)

// Valid reports whether t is a known transport.
func (t SIPTransport) Valid() bool {
	return t == SIPTransportUDP || t == SIPTransportTCP || t == SIPTransportTLS
}

// SIPClient is a device's SIP (VoIP) account: the payload of a
// ModuleSIPClient module. Every field may be empty; Port 0 means the
// transport's default port. Extension is kept without separators. The
// account password is deliberately not stored.
type SIPClient struct {
	// Server is the registrar or proxy: a host name or IP address.
	Server    string
	Port      int
	Transport SIPTransport
	Username  string
	Extension string
}

// String describes c in one line, for the audit log.
func (c SIPClient) String() string {
	server := c.Server
	if c.Port != 0 {
		server += ":" + strconv.Itoa(c.Port)
	}

	return strings.Join([]string{server, string(c.Transport), c.Username, c.Extension}, " / ")
}

// SIPClientParams is unvalidated input for a SIP client module.
type SIPClientParams struct {
	Server    string
	Port      int
	Transport SIPTransport
	Username  string
	Extension string
}

func checkSIPClient(f fieldErrors, field string, p SIPClientParams) SIPClient {
	if p.Port < 0 || p.Port > maxPort {
		f.add(field+".port", FieldInvalid)
	}

	if p.Transport != "" && !p.Transport.Valid() {
		f.add(field+".transport", FieldInvalid)
	}

	return SIPClient{
		Server:    checkSIPServer(f, field+".server", p.Server),
		Port:      p.Port,
		Transport: p.Transport,
		Username:  checkOptionalText(f, field+".username", p.Username, maxShortTextRunes),
		Extension: checkNumber(f, field+".extension", p.Extension, sipExtensionPattern),
	}
}

// checkSIPServer trims s and checks it is empty, an RFC 1123 host name or
// an IP address (IPv6 without brackets or zone).
func checkSIPServer(f fieldErrors, field, s string) string {
	s = strings.TrimSpace(s)

	if addr, err := netip.ParseAddr(s); err == nil {
		if addr.Zone() != "" {
			f.add(field, FieldInvalid)
		}

		return addr.String()
	}

	return checkHostname(f, field, s)
}
