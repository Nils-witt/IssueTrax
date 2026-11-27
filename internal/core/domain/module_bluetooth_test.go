package domain_test

import (
	"strings"
	"testing"

	"nilswitt.dev/issuetrax/internal/core/domain"
)

func TestBluetoothModule(t *testing.T) {
	t.Parallel()

	const field = "modules[0].bluetooth."

	tests := []struct {
		name   string
		params domain.BluetoothParams
		want   domain.Bluetooth
		errs   map[string]string
	}{
		{
			name:   "all fields, trimmed and address normalized",
			params: domain.BluetoothParams{Name: " Desk Phone ", Address: " AA-BB-CC-00-11-22 ", Version: " 5.3 "},
			want:   domain.Bluetooth{Name: "Desk Phone", Address: "aa:bb:cc:00:11:22", Version: "5.3"},
		},
		{name: "empty", params: domain.BluetoothParams{}, want: domain.Bluetooth{}},
		{
			name:   "dotted address",
			params: domain.BluetoothParams{Address: "aabb.cc00.1122", Version: "10.1"},
			want:   domain.Bluetooth{Address: "aa:bb:cc:00:11:22", Version: "10.1"},
		},
		{
			name:   "name too long",
			params: domain.BluetoothParams{Name: strings.Repeat("a", 101)},
			errs:   map[string]string{field + "name": domain.FieldTooLong},
		},
		{
			name:   "invalid address",
			params: domain.BluetoothParams{Address: "aa:bb:cc"},
			errs:   map[string]string{field + "address": domain.FieldInvalid},
		},
		{
			name:   "EUI-64 address",
			params: domain.BluetoothParams{Address: "aa:bb:cc:dd:ee:ff:00:11"},
			errs:   map[string]string{field + "address": domain.FieldInvalid},
		},
		{
			name:   "version without minor",
			params: domain.BluetoothParams{Version: "5"},
			errs:   map[string]string{field + "version": domain.FieldInvalid},
		},
		{
			name:   "version with text",
			params: domain.BluetoothParams{Version: "5.3 LE"},
			errs:   map[string]string{field + "version": domain.FieldInvalid},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			item, err := itemWith(domain.ModuleParams{Kind: domain.ModuleBluetooth, Bluetooth: &tt.params})
			wantFields(t, err, tt.errs)

			if err == nil && *item.Modules[0].Bluetooth != tt.want {
				t.Fatalf("bluetooth = %+v, want %+v", *item.Modules[0].Bluetooth, tt.want)
			}
		})
	}
}

func TestBluetoothModuleIsSingle(t *testing.T) {
	t.Parallel()

	p := validItem()
	p.Modules = []domain.ModuleParams{
		{Kind: domain.ModuleBluetooth, Bluetooth: &domain.BluetoothParams{}},
		{Kind: domain.ModuleBluetooth, Bluetooth: &domain.BluetoothParams{}},
	}

	_, err := domain.NewStorageItem(p)
	wantFields(t, err, map[string]string{"modules[1].kind": domain.FieldInvalid})
}

func TestBluetoothString(t *testing.T) {
	t.Parallel()

	b := domain.Bluetooth{Name: "Phone", Address: "aa:bb:cc:00:11:22", Version: "5.3"}
	if got, want := b.String(), "Phone / aa:bb:cc:00:11:22 / 5.3"; got != want {
		t.Fatalf("String() = %q, want %q", got, want)
	}
}
