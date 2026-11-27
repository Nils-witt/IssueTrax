package domain_test

import (
	"strings"
	"testing"

	"nilswitt.dev/issuetrax/internal/core/domain"
)

func TestWWANModule(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		params domain.WWANParams
		want   domain.WWAN
		errs   map[string]string
	}{
		{
			name:   "all fields, separators removed",
			params: domain.WWANParams{IMEI: "49-015420-323751-8", APN: " internet.telekom ", Technology: domain.WWAN5G},
			want:   domain.WWAN{IMEI: "490154203237518", APN: "internet.telekom", Technology: domain.WWAN5G},
		},
		{name: "empty", params: domain.WWANParams{}, want: domain.WWAN{}},
		{
			name:   "imei with wrong check digit",
			params: domain.WWANParams{IMEI: "490154203237519"},
			errs:   map[string]string{"modules[0].wwan.imei": domain.FieldInvalid},
		},
		{
			name:   "imei too short",
			params: domain.WWANParams{IMEI: "49015420323751"},
			errs:   map[string]string{"modules[0].wwan.imei": domain.FieldInvalid},
		},
		{
			name:   "imei with letters",
			params: domain.WWANParams{IMEI: "49015420323751A"},
			errs:   map[string]string{"modules[0].wwan.imei": domain.FieldInvalid},
		},
		{
			name:   "apn too long",
			params: domain.WWANParams{APN: strings.Repeat("a", 101)},
			errs:   map[string]string{"modules[0].wwan.apn": domain.FieldTooLong},
		},
		{
			name:   "unknown technology",
			params: domain.WWANParams{Technology: "6g"},
			errs:   map[string]string{"modules[0].wwan.technology": domain.FieldInvalid},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			item, err := itemWith(domain.ModuleParams{Kind: domain.ModuleWWAN, WWAN: &tt.params})
			wantFields(t, err, tt.errs)

			if err == nil && *item.Modules[0].WWAN != tt.want {
				t.Fatalf("wwan = %+v, want %+v", *item.Modules[0].WWAN, tt.want)
			}
		})
	}
}

func TestWWANModuleIsSingle(t *testing.T) {
	t.Parallel()

	p := validItem()
	p.Modules = []domain.ModuleParams{
		{Kind: domain.ModuleWWAN, WWAN: &domain.WWANParams{}},
		{Kind: domain.ModuleWWAN, WWAN: &domain.WWANParams{}},
	}

	_, err := domain.NewStorageItem(p)
	wantFields(t, err, map[string]string{"modules[1].kind": domain.FieldInvalid})
}
