package domain_test

import (
	"strings"
	"testing"

	"nilswitt.dev/issuetrax/internal/core/domain"
)

func TestSimCardModule(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		params domain.SimCardParams
		want   domain.SimCard
		errs   map[string]string
	}{
		{
			name: "all fields, separators removed",
			params: domain.SimCardParams{
				Provider: " Telekom ", PhoneNumber: " +49 (171) 123-4567 ", ICCID: "8949 0200 0012 3456 7890",
			},
			want: domain.SimCard{Provider: "Telekom", PhoneNumber: "+491711234567", ICCID: "89490200001234567890"},
		},
		{name: "empty", params: domain.SimCardParams{}, want: domain.SimCard{}},
		{
			name:   "national number",
			params: domain.SimCardParams{PhoneNumber: "0171/1234567"},
			want:   domain.SimCard{PhoneNumber: "01711234567"},
		},
		{
			name:   "phone number with letters",
			params: domain.SimCardParams{PhoneNumber: "0171 CALLME"},
			errs:   map[string]string{"modules[0].simCard.phoneNumber": domain.FieldInvalid},
		},
		{
			name:   "phone number too long",
			params: domain.SimCardParams{PhoneNumber: "+" + strings.Repeat("1", 16)},
			errs:   map[string]string{"modules[0].simCard.phoneNumber": domain.FieldInvalid},
		},
		{
			name:   "iccid without industry prefix",
			params: domain.SimCardParams{ICCID: "12490200001234567890"},
			errs:   map[string]string{"modules[0].simCard.iccid": domain.FieldInvalid},
		},
		{
			name:   "iccid too short",
			params: domain.SimCardParams{ICCID: "8949020000123456"},
			errs:   map[string]string{"modules[0].simCard.iccid": domain.FieldInvalid},
		},
		{
			name:   "provider too long",
			params: domain.SimCardParams{Provider: strings.Repeat("a", 101)},
			errs:   map[string]string{"modules[0].simCard.provider": domain.FieldTooLong},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			item, err := itemWith(domain.ModuleParams{Kind: domain.ModuleSimCard, SimCard: &tt.params})
			wantFields(t, err, tt.errs)

			if err == nil && *item.Modules[0].SimCard != tt.want {
				t.Fatalf("sim card = %+v, want %+v", *item.Modules[0].SimCard, tt.want)
			}
		})
	}
}

func TestSimCardModuleIsSingle(t *testing.T) {
	t.Parallel()

	p := validItem()
	p.Modules = []domain.ModuleParams{
		{Kind: domain.ModuleSimCard, SimCard: &domain.SimCardParams{}},
		{Kind: domain.ModuleSimCard, SimCard: &domain.SimCardParams{}},
	}

	_, err := domain.NewStorageItem(p)
	wantFields(t, err, map[string]string{"modules[1].kind": domain.FieldInvalid})
}
