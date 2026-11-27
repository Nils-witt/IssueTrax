package httpapi

import (
	"strconv"
	"time"

	"github.com/google/uuid"

	"nilswitt.dev/issuetrax/internal/core/domain"
	"nilswitt.dev/issuetrax/internal/core/port"
)

type ipConfigJSON struct {
	Mode string `json:"mode"`
	// Address, PrefixLength and Gateway are null unless set (static mode).
	Address      *string  `json:"address"`
	PrefixLength *int     `json:"prefixLength"`
	Gateway      *string  `json:"gateway"`
	DNSServers   []string `json:"dnsServers"`
}

type networkInterfaceJSON struct {
	Name string `json:"name"`
	MAC  string `json:"mac"`
	Type string `json:"type"`
	SSID string `json:"ssid"`
	// NativeVLAN is null when the interface has no untagged VLAN.
	NativeVLAN  *int         `json:"nativeVlan"`
	TaggedVLANs []int        `json:"taggedVlans"`
	IPv4        ipConfigJSON `json:"ipv4"`
	IPv6        ipConfigJSON `json:"ipv6"`
}

// hardwareJSON names a device. Requests set the IDs; responses also carry
// the named entries (null when unset) so clients need no extra lookups.
type hardwareJSON struct {
	ManufacturerID *string         `json:"manufacturerId"`
	ModelID        *string         `json:"modelId"`
	SerialNumber   string          `json:"serialNumber"`
	Manufacturer   *catalogRefJSON `json:"manufacturer"`
	Model          *catalogRefJSON `json:"model"`
}

type networkHostJSON struct {
	Role     string `json:"role"`
	Hostname string `json:"hostname"`
}

type purchaseJSON struct {
	Supplier      string `json:"supplier"`
	InvoiceNumber string `json:"invoiceNumber"`
	// Price is a decimal string with at most two decimals ("12.50"); null
	// when unknown. Currency is an ISO 4217 code, empty without a price.
	Price       *string    `json:"price"`
	Currency    string     `json:"currency"`
	WarrantyEnd *time.Time `json:"warrantyEnd"`
}

// moduleJSON is one module instance. Exactly the payload matching kind is
// set. On requests, id names an instance to keep; leave it out for a new one.
type moduleJSON struct {
	ID               *string               `json:"id,omitempty"`
	Kind             string                `json:"kind"`
	Primary          bool                  `json:"primary"`
	Hardware         *hardwareJSON         `json:"hardware,omitempty"`
	NetworkHost      *networkHostJSON      `json:"networkHost,omitempty"`
	Purchase         *purchaseJSON         `json:"purchase,omitempty"`
	NetworkInterface *networkInterfaceJSON `json:"networkInterface,omitempty"`
}

// toModuleJSON converts m; v resolves the hardware's manufacturer and model.
func toModuleJSON(m domain.Module, v port.StorageItemView) moduleJSON {
	id := m.ID.String()
	resp := moduleJSON{ID: &id, Kind: string(m.Kind), Primary: m.Primary}

	switch {
	case m.Hardware != nil:
		resp.Hardware = toHardwareJSON(*m.Hardware, v)
	case m.NetworkHost != nil:
		resp.NetworkHost = &networkHostJSON{Role: string(m.NetworkHost.Role), Hostname: m.NetworkHost.Hostname}
	case m.Purchase != nil:
		p := m.Purchase
		resp.Purchase = &purchaseJSON{
			Supplier: p.Supplier, InvoiceNumber: p.InvoiceNumber, Currency: p.Currency, WarrantyEnd: p.WarrantyEnd,
		}

		if p.PriceCents != nil {
			resp.Purchase.Price = new(domain.FormatPrice(*p.PriceCents))
		}
	case m.NetworkInterface != nil:
		resp.NetworkInterface = new(toNetworkInterfaceJSON(*m.NetworkInterface))
	}

	return resp
}

// toModulesJSON converts the modules of v.
func toModulesJSON(v port.StorageItemView) []moduleJSON {
	out := make([]moduleJSON, 0, len(v.Modules))
	for _, m := range v.Modules {
		out = append(out, toModuleJSON(m, v))
	}

	return out
}

func toHardwareJSON(h domain.Hardware, v port.StorageItemView) *hardwareJSON {
	resp := &hardwareJSON{SerialNumber: h.SerialNumber}

	if h.ManufacturerID != nil {
		resp.ManufacturerID = new(h.ManufacturerID.String())
		if v.Manufacturer != nil {
			resp.Manufacturer = new(toManufacturerRef(*v.Manufacturer))
		}
	}

	if h.ModelID != nil {
		resp.ModelID = new(h.ModelID.String())
		if v.Model != nil {
			resp.Model = new(toHardwareModelRef(*v.Model))
		}
	}

	return resp
}

func toNetworkInterfaceJSON(n domain.NetworkInterface) networkInterfaceJSON {
	resp := networkInterfaceJSON{
		Name:        n.Name,
		MAC:         n.MAC,
		Type:        string(n.Type),
		SSID:        n.SSID,
		TaggedVLANs: append([]int{}, n.TaggedVLANs...),
		IPv4:        toIPConfigJSON(n.IPv4),
		IPv6:        toIPConfigJSON(n.IPv6),
	}

	if n.NativeVLAN != 0 {
		resp.NativeVLAN = new(n.NativeVLAN)
	}

	return resp
}

func toIPConfigJSON(c domain.IPConfig) ipConfigJSON {
	resp := ipConfigJSON{Mode: string(c.Mode), DNSServers: make([]string, 0, len(c.DNSServers))}

	if c.Address.IsValid() {
		resp.Address, resp.PrefixLength = new(c.Address.Addr().String()), new(c.Address.Bits())
	}

	if c.Gateway.IsValid() {
		resp.Gateway = new(c.Gateway.String())
	}

	for _, d := range c.DNSServers {
		resp.DNSServers = append(resp.DNSServers, d.String())
	}

	return resp
}

// moduleParams converts request modules. An unparsable id is reported on
// modules[i].id.
func moduleParams(modules []moduleJSON) ([]domain.ModuleParams, error) {
	out := make([]domain.ModuleParams, 0, len(modules))

	for i, m := range modules {
		p := domain.ModuleParams{Kind: domain.ModuleKind(m.Kind), Primary: m.Primary}

		if m.ID != nil {
			id, err := uuid.Parse(*m.ID)
			if err != nil {
				return nil, domain.NewValidationError(map[string]string{"modules[" + strconv.Itoa(i) + "].id": domain.FieldInvalid})
			}

			p.ID = new(domain.ModuleID(id))
		}

		if h := m.Hardware; h != nil {
			hw, err := h.params("modules[" + strconv.Itoa(i) + "].hardware")
			if err != nil {
				return nil, err
			}

			p.Hardware = &hw
		}

		if h := m.NetworkHost; h != nil {
			p.NetworkHost = &domain.NetworkHostParams{Role: domain.NetworkRole(h.Role), Hostname: h.Hostname}
		}

		if pu := m.Purchase; pu != nil {
			p.Purchase = &domain.PurchaseParams{
				Supplier: pu.Supplier, InvoiceNumber: pu.InvoiceNumber, Price: deref(pu.Price), Currency: pu.Currency,
				WarrantyEnd: pu.WarrantyEnd,
			}
		}

		if n := m.NetworkInterface; n != nil {
			p.NetworkInterface = new(n.params())
		}

		out = append(out, p)
	}

	return out, nil
}

// params converts h; an unparsable ID is reported below field.
func (h hardwareJSON) params(field string) (domain.HardwareParams, error) {
	p := domain.HardwareParams{SerialNumber: h.SerialNumber}

	if h.ManufacturerID != nil {
		id, err := uuid.Parse(*h.ManufacturerID)
		if err != nil {
			return p, domain.NewValidationError(map[string]string{field + ".manufacturerId": domain.FieldInvalid})
		}

		p.ManufacturerID = new(domain.ManufacturerID(id))
	}

	if h.ModelID != nil {
		id, err := uuid.Parse(*h.ModelID)
		if err != nil {
			return p, domain.NewValidationError(map[string]string{field + ".modelId": domain.FieldInvalid})
		}

		p.ModelID = new(domain.HardwareModelID(id))
	}

	return p, nil
}

func (n networkInterfaceJSON) params() domain.NetworkInterfaceParams {
	return domain.NetworkInterfaceParams{
		Name:        n.Name,
		MAC:         n.MAC,
		Type:        domain.NetworkInterfaceType(n.Type),
		SSID:        n.SSID,
		NativeVLAN:  n.NativeVLAN,
		TaggedVLANs: n.TaggedVLANs,
		IPv4:        n.IPv4.params(),
		IPv6:        n.IPv6.params(),
	}
}

func (c ipConfigJSON) params() domain.IPConfigParams {
	return domain.IPConfigParams{
		Mode:         domain.IPMode(c.Mode),
		Address:      deref(c.Address),
		PrefixLength: c.PrefixLength,
		Gateway:      deref(c.Gateway),
		DNSServers:   c.DNSServers,
	}
}

type moduleDefaultJSON struct {
	Module string `json:"module"`
	Count  int    `json:"count"`
}

func toModuleDefaultsJSON(defaults []domain.ModuleDefault) []moduleDefaultJSON {
	out := make([]moduleDefaultJSON, 0, len(defaults))
	for _, d := range defaults {
		out = append(out, moduleDefaultJSON{Module: string(d.Module), Count: d.Count})
	}

	return out
}

func moduleDefaults(defaults []moduleDefaultJSON) []domain.ModuleDefault {
	out := make([]domain.ModuleDefault, 0, len(defaults))
	for _, d := range defaults {
		out = append(out, domain.ModuleDefault{Module: domain.ModuleKind(d.Module), Count: d.Count})
	}

	return out
}

func deref(s *string) string {
	if s == nil {
		return ""
	}

	return *s
}
