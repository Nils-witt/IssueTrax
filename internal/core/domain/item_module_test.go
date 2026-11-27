package domain_test

import (
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"

	"nilswitt.dev/issuetrax/internal/core/domain"
)

func iface(name string) *domain.NetworkInterfaceParams {
	p := dhcpInterface()
	p.Name = name

	return &p
}

func TestSortModules(t *testing.T) {
	t.Parallel()

	mod := func(kind domain.ModuleKind, name string, primary bool) domain.Module {
		m := domain.Module{ID: domain.ModuleID(uuid.New()), Kind: kind, Primary: primary}
		if kind == domain.ModuleNetworkInterface {
			m.NetworkInterface = &domain.NetworkInterface{Name: name}
		}

		return m
	}

	modules := []domain.Module{
		mod(domain.ModuleNetworkInterface, "eth10", false),
		mod(domain.ModulePurchase, "", false),
		mod(domain.ModuleNetworkInterface, "ETH2", false),
		mod(domain.ModuleNetworkInterface, "wlan0", true),
		mod(domain.ModuleHardware, "", false),
		mod(domain.ModuleNetworkInterface, "eth02a", false),
		mod(domain.ModuleNetworkHost, "", false),
	}

	domain.SortModules(modules)

	got := make([]string, 0, len(modules))
	for _, m := range modules {
		label := string(m.Kind)
		if m.NetworkInterface != nil {
			label = m.NetworkInterface.Name
		}

		got = append(got, label)
	}

	want := []string{"hardware", "network_host", "purchase", "wlan0", "ETH2", "eth02a", "eth10"}
	if !slices.Equal(got, want) {
		t.Fatalf("SortModules() = %v, want %v", got, want)
	}
}

func newModuleID() domain.ModuleID { return domain.ModuleID(uuid.New()) }

func TestStorageItemModules(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		modules    []domain.ModuleParams
		wantFields map[string]string
	}{
		{name: "none"},
		{
			name: "all kinds",
			modules: []domain.ModuleParams{
				{Kind: domain.ModuleNetworkInterface, NetworkInterface: iface("eth1")},
				{Kind: domain.ModuleNetworkInterface, Primary: true, NetworkInterface: iface("eth0")},
				{Kind: domain.ModuleHardware, Hardware: &domain.HardwareParams{Manufacturer: " ACME "}},
				{Kind: domain.ModuleNetworkHost, NetworkHost: &domain.NetworkHostParams{Role: domain.NetworkRoleSwitch, Hostname: "sw1"}},
				{Kind: domain.ModulePurchase, Purchase: &domain.PurchaseParams{Price: "12.5", Currency: "eur"}},
			},
		},
		{
			name:       "missing kind",
			modules:    []domain.ModuleParams{{}},
			wantFields: map[string]string{"modules[0].kind": domain.FieldRequired},
		},
		{
			name:       "unknown kind",
			modules:    []domain.ModuleParams{{Kind: "nope"}},
			wantFields: map[string]string{"modules[0].kind": domain.FieldInvalid},
		},
		{
			name:       "missing payload",
			modules:    []domain.ModuleParams{{Kind: domain.ModuleHardware}},
			wantFields: map[string]string{"modules[0].hardware": domain.FieldRequired},
		},
		{
			name: "wrong payload",
			modules: []domain.ModuleParams{{
				Kind: domain.ModuleHardware, Hardware: &domain.HardwareParams{}, Purchase: &domain.PurchaseParams{},
			}},
			wantFields: map[string]string{"modules[0].purchase": domain.FieldInvalid},
		},
		{
			name: "single twice",
			modules: []domain.ModuleParams{
				{Kind: domain.ModuleHardware, Hardware: &domain.HardwareParams{}},
				{Kind: domain.ModuleHardware, Hardware: &domain.HardwareParams{}},
			},
			wantFields: map[string]string{"modules[1].kind": domain.FieldInvalid},
		},
		{
			name:       "primary single",
			modules:    []domain.ModuleParams{{Kind: domain.ModuleHardware, Primary: true, Hardware: &domain.HardwareParams{}}},
			wantFields: map[string]string{"modules[0].primary": domain.FieldInvalid},
		},
		{
			name: "two primaries",
			modules: []domain.ModuleParams{
				{Kind: domain.ModuleNetworkInterface, Primary: true, NetworkInterface: iface("eth0")},
				{Kind: domain.ModuleNetworkInterface, Primary: true, NetworkInterface: iface("eth1")},
			},
			wantFields: map[string]string{"modules[1].primary": domain.FieldInvalid},
		},
		{
			name: "unknown id",
			modules: []domain.ModuleParams{
				{ID: new(newModuleID()), Kind: domain.ModuleHardware, Hardware: &domain.HardwareParams{}},
			},
			wantFields: map[string]string{"modules[0].id": domain.FieldInvalid},
		},
		{
			name: "payload errors",
			modules: []domain.ModuleParams{
				{Kind: domain.ModuleNetworkHost, NetworkHost: &domain.NetworkHostParams{Hostname: "-bad"}},
				{Kind: domain.ModulePurchase, Purchase: &domain.PurchaseParams{Price: "1.234"}},
				{Kind: domain.ModuleNetworkInterface, NetworkInterface: &domain.NetworkInterfaceParams{
					Name: "eth0", MAC: "nope", Type: domain.NetworkInterfaceWired,
					IPv4: domain.IPConfigParams{Mode: domain.IPModeDHCP}, IPv6: domain.IPConfigParams{Mode: domain.IPModeOff},
				}},
			},
			wantFields: map[string]string{
				"modules[0].networkHost.role":     domain.FieldRequired,
				"modules[0].networkHost.hostname": domain.FieldInvalid,
				"modules[1].purchase.price":       domain.FieldInvalid,
				"modules[1].purchase.currency":    domain.FieldRequired,
				"modules[2].networkInterface.mac": domain.FieldInvalid,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			p := validItem()
			p.Modules = tt.modules

			item, err := domain.NewStorageItem(p)
			wantFields(t, err, tt.wantFields)

			if err == nil && len(item.Modules) != len(tt.modules) {
				t.Fatalf("modules = %v", item.Modules)
			}
		})
	}
}

func TestPurchaseModule(t *testing.T) {
	t.Parallel()

	warranty := testNow.Add(365 * 24 * time.Hour)

	tests := []struct {
		name       string
		params     domain.PurchaseParams
		wantCents  *int64
		wantCur    string
		wantFields map[string]string
	}{
		{name: "empty"},
		{name: "whole", params: domain.PurchaseParams{Price: "12", Currency: "usd"}, wantCents: new(int64(1200)), wantCur: "USD"},
		{name: "one decimal", params: domain.PurchaseParams{Price: "0.5", Currency: "EUR"}, wantCents: new(int64(50)), wantCur: "EUR"},
		{name: "two decimals", params: domain.PurchaseParams{Price: "1999.99", Currency: "EUR", WarrantyEnd: &warranty}, wantCents: new(int64(199999)), wantCur: "EUR"},
		{name: "negative", params: domain.PurchaseParams{Price: "-1", Currency: "EUR"}, wantFields: map[string]string{"modules[0].purchase.price": domain.FieldInvalid}},
		{name: "three decimals", params: domain.PurchaseParams{Price: "1.001", Currency: "EUR"}, wantFields: map[string]string{"modules[0].purchase.price": domain.FieldInvalid}},
		{name: "unknown currency", params: domain.PurchaseParams{Price: "1", Currency: "EUX"}, wantFields: map[string]string{"modules[0].purchase.currency": domain.FieldInvalid}},
		{name: "currency without price", params: domain.PurchaseParams{Currency: "EUR"}, wantFields: map[string]string{"modules[0].purchase.currency": domain.FieldInvalid}},
		{name: "price without currency", params: domain.PurchaseParams{Price: "1"}, wantFields: map[string]string{"modules[0].purchase.currency": domain.FieldRequired}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			p := validItem()
			p.Modules = []domain.ModuleParams{{Kind: domain.ModulePurchase, Purchase: &tt.params}}

			item, err := domain.NewStorageItem(p)
			wantFields(t, err, tt.wantFields)

			if err != nil {
				return
			}

			got := item.Modules[0].Purchase
			if got.Currency != tt.wantCur || (got.PriceCents == nil) != (tt.wantCents == nil) ||
				(got.PriceCents != nil && *got.PriceCents != *tt.wantCents) {
				t.Fatalf("purchase = %+v", got)
			}
		})
	}

	if got := domain.FormatPrice(199999); got != "1999.99" {
		t.Fatalf("FormatPrice = %q", got)
	}
}
