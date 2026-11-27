package domain_test

import (
	"strings"
	"testing"

	"nilswitt.dev/issuetrax/internal/core/domain"
)

func TestOperatingSystemModule(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		params domain.OperatingSystemParams
		want   domain.OperatingSystem
		errs   map[string]string
	}{
		{
			name:   "all fields, trimmed",
			params: domain.OperatingSystemParams{Name: " Debian ", Version: " 12 "},
			want:   domain.OperatingSystem{Name: "Debian", Version: "12"},
		},
		{name: "empty", params: domain.OperatingSystemParams{}, want: domain.OperatingSystem{}},
		{
			name:   "name too long",
			params: domain.OperatingSystemParams{Name: strings.Repeat("a", 101)},
			errs:   map[string]string{"modules[0].operatingSystem.name": domain.FieldTooLong},
		},
		{
			name:   "version too long",
			params: domain.OperatingSystemParams{Version: strings.Repeat("1", 101)},
			errs:   map[string]string{"modules[0].operatingSystem.version": domain.FieldTooLong},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			item, err := itemWith(domain.ModuleParams{Kind: domain.ModuleOperatingSystem, OperatingSystem: &tt.params})
			wantFields(t, err, tt.errs)

			if err == nil && *item.Modules[0].OperatingSystem != tt.want {
				t.Fatalf("operating system = %+v, want %+v", *item.Modules[0].OperatingSystem, tt.want)
			}
		})
	}
}

func TestOperatingSystemModuleIsSingle(t *testing.T) {
	t.Parallel()

	p := validItem()
	p.Modules = []domain.ModuleParams{
		{Kind: domain.ModuleOperatingSystem, OperatingSystem: &domain.OperatingSystemParams{}},
		{Kind: domain.ModuleOperatingSystem, OperatingSystem: &domain.OperatingSystemParams{}},
	}

	_, err := domain.NewStorageItem(p)
	wantFields(t, err, map[string]string{"modules[1].kind": domain.FieldInvalid})
}
