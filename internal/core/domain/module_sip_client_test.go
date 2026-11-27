package domain_test

import (
	"strings"
	"testing"

	"nilswitt.dev/issuetrax/internal/core/domain"
)

func TestSIPClientModule(t *testing.T) {
	t.Parallel()

	const field = "modules[0].sipClient."

	tests := []struct {
		name   string
		params domain.SIPClientParams
		want   domain.SIPClient
		errs   map[string]string
	}{
		{
			name: "all fields, trimmed and separators removed",
			params: domain.SIPClientParams{
				Server: " pbx.example.org ", Port: 5061, Transport: domain.SIPTransportTLS, Username: " ada ",
				Extension: "+49 30 123-456",
			},
			want: domain.SIPClient{
				Server: "pbx.example.org", Port: 5061, Transport: domain.SIPTransportTLS, Username: "ada",
				Extension: "+4930123456",
			},
		},
		{name: "empty", params: domain.SIPClientParams{}, want: domain.SIPClient{}},
		{
			name:   "ipv4 server",
			params: domain.SIPClientParams{Server: "10.0.0.5", Extension: "7"},
			want:   domain.SIPClient{Server: "10.0.0.5", Extension: "7"},
		},
		{
			name:   "ipv6 server normalized",
			params: domain.SIPClientParams{Server: "2001:DB8::0:1"},
			want:   domain.SIPClient{Server: "2001:db8::1"},
		},
		{
			name:   "server with port",
			params: domain.SIPClientParams{Server: "pbx.example.org:5060"},
			errs:   map[string]string{field + "server": domain.FieldInvalid},
		},
		{
			name:   "ipv6 server with zone",
			params: domain.SIPClientParams{Server: "fe80::1%eth0"},
			errs:   map[string]string{field + "server": domain.FieldInvalid},
		},
		{
			name:   "server too long",
			params: domain.SIPClientParams{Server: strings.Repeat("a.", 127) + "a"},
			errs:   map[string]string{field + "server": domain.FieldTooLong},
		},
		{
			name:   "negative port",
			params: domain.SIPClientParams{Port: -1},
			errs:   map[string]string{field + "port": domain.FieldInvalid},
		},
		{
			name:   "port too large",
			params: domain.SIPClientParams{Port: 65536},
			errs:   map[string]string{field + "port": domain.FieldInvalid},
		},
		{
			name:   "unknown transport",
			params: domain.SIPClientParams{Transport: "sctp"},
			errs:   map[string]string{field + "transport": domain.FieldInvalid},
		},
		{
			name:   "username too long",
			params: domain.SIPClientParams{Username: strings.Repeat("a", 101)},
			errs:   map[string]string{field + "username": domain.FieldTooLong},
		},
		{
			name:   "extension with letters",
			params: domain.SIPClientParams{Extension: "ada"},
			errs:   map[string]string{field + "extension": domain.FieldInvalid},
		},
		{
			name:   "extension too long",
			params: domain.SIPClientParams{Extension: "1234567890123456"},
			errs:   map[string]string{field + "extension": domain.FieldInvalid},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			item, err := itemWith(domain.ModuleParams{Kind: domain.ModuleSIPClient, SIPClient: &tt.params})
			wantFields(t, err, tt.errs)

			if err == nil && *item.Modules[0].SIPClient != tt.want {
				t.Fatalf("sip client = %+v, want %+v", *item.Modules[0].SIPClient, tt.want)
			}
		})
	}
}

func TestSIPClientModuleIsSingle(t *testing.T) {
	t.Parallel()

	p := validItem()
	p.Modules = []domain.ModuleParams{
		{Kind: domain.ModuleSIPClient, SIPClient: &domain.SIPClientParams{}},
		{Kind: domain.ModuleSIPClient, SIPClient: &domain.SIPClientParams{}},
	}

	_, err := domain.NewStorageItem(p)
	wantFields(t, err, map[string]string{"modules[1].kind": domain.FieldInvalid})
}

func TestSIPClientString(t *testing.T) {
	t.Parallel()

	c := domain.SIPClient{Server: "pbx", Port: 5060, Transport: domain.SIPTransportUDP, Username: "ada", Extension: "42"}
	if got, want := c.String(), "pbx:5060 / udp / ada / 42"; got != want {
		t.Fatalf("String() = %q, want %q", got, want)
	}
}
