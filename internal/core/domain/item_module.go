package domain

import (
	"cmp"
	"fmt"
	"slices"
	"strings"
	"unicode"

	"github.com/google/uuid"
)

// ModuleID identifies one module instance of a storage item. It stays the
// same across updates of the item.
type ModuleID uuid.UUID

func (id ModuleID) String() string { return uuid.UUID(id).String() }

// ModuleKind names a module: a group of fields an item can carry. Modules
// are defined in code; each kind has its own payload type.
type ModuleKind string

// Module kinds, in display order.
const (
	ModuleHardware         ModuleKind = "hardware"
	ModuleNetworkHost      ModuleKind = "network_host"
	ModulePurchase         ModuleKind = "purchase"
	ModuleSimCard          ModuleKind = "sim_card"
	ModuleWWAN             ModuleKind = "wwan"
	ModuleOperatingSystem  ModuleKind = "operating_system"
	ModuleNetworkInterface ModuleKind = "network_interface"
)

// ModuleKinds lists every module kind in display order.
var ModuleKinds = []ModuleKind{
	ModuleHardware, ModuleNetworkHost, ModulePurchase, ModuleSimCard, ModuleWWAN, ModuleOperatingSystem,
	ModuleNetworkInterface,
}

// Valid reports whether k is a known module kind.
func (k ModuleKind) Valid() bool { return slices.Contains(ModuleKinds, k) }

// Multi reports whether an item may carry k more than once. Only multi
// modules can mark an instance as primary.
func (k ModuleKind) Multi() bool { return k == ModuleNetworkInterface }

// order is k's position in the display order.
func (k ModuleKind) order() int { return slices.Index(ModuleKinds, k) }

// Module is one module instance of a storage item. Exactly the payload
// field matching Kind is set.
type Module struct {
	ID   ModuleID
	Kind ModuleKind
	// Primary marks the main instance of a multi module; at most one
	// instance per kind is primary. Always false for single modules.
	Primary bool

	Hardware         *Hardware
	NetworkHost      *NetworkHost
	Purchase         *Purchase
	SimCard          *SimCard
	WWAN             *WWAN
	OperatingSystem  *OperatingSystem
	NetworkInterface *NetworkInterface
}

// Equal reports whether m and o are the same instance with the same content.
func (m Module) Equal(o Module) bool {
	return m.ID == o.ID && m.Kind == o.Kind && m.Primary == o.Primary &&
		equalPtr(m.Hardware, o.Hardware, Hardware.Equal) &&
		equalPtr(m.NetworkHost, o.NetworkHost, func(a, b NetworkHost) bool { return a == b }) &&
		equalPtr(m.Purchase, o.Purchase, Purchase.Equal) &&
		equalPtr(m.SimCard, o.SimCard, func(a, b SimCard) bool { return a == b }) &&
		equalPtr(m.WWAN, o.WWAN, func(a, b WWAN) bool { return a == b }) &&
		equalPtr(m.OperatingSystem, o.OperatingSystem, func(a, b OperatingSystem) bool { return a == b }) &&
		equalPtr(m.NetworkInterface, o.NetworkInterface, NetworkInterface.Equal)
}

// String describes m in one line, for the audit log.
func (m Module) String() string {
	var payload string

	switch {
	case m.Hardware != nil:
		payload = m.Hardware.String()
	case m.NetworkHost != nil:
		payload = m.NetworkHost.String()
	case m.Purchase != nil:
		payload = m.Purchase.String()
	case m.SimCard != nil:
		payload = m.SimCard.String()
	case m.WWAN != nil:
		payload = m.WWAN.String()
	case m.OperatingSystem != nil:
		payload = m.OperatingSystem.String()
	case m.NetworkInterface != nil:
		payload = m.NetworkInterface.String()
	}

	s := string(m.Kind)
	if m.Primary {
		s += " (primary)"
	}

	return s + ": " + payload
}

// name is what instances of one kind are sorted by.
func (m Module) name() string {
	if m.NetworkInterface != nil {
		return m.NetworkInterface.Name
	}

	return ""
}

// ModuleParams is unvalidated input for one module instance. ID names an
// existing instance of the item to keep (nil for a new one); exactly the
// payload matching Kind must be set.
type ModuleParams struct {
	ID      *ModuleID
	Kind    ModuleKind
	Primary bool

	Hardware         *HardwareParams
	NetworkHost      *NetworkHostParams
	Purchase         *PurchaseParams
	SimCard          *SimCardParams
	WWAN             *WWANParams
	OperatingSystem  *OperatingSystemParams
	NetworkInterface *NetworkInterfaceParams
}

// SortModules sorts modules into display order: by kind, then the primary
// instance first, then by name in natural order ("eth2" before "eth10"),
// then by ID.
func SortModules(modules []Module) {
	slices.SortFunc(modules, func(a, b Module) int {
		if c := cmp.Compare(a.Kind.order(), b.Kind.order()); c != 0 {
			return c
		}

		if a.Primary != b.Primary {
			if a.Primary {
				return -1
			}

			return 1
		}

		if c := naturalCompare(a.name(), b.name()); c != 0 {
			return c
		}

		return strings.Compare(a.ID.String(), b.ID.String())
	})
}

// checkModules validates ps against the item's current modules and returns
// them in display order. Instances with an ID must be among existing and
// keep their kind; instances without one get newID.
func checkModules(f fieldErrors, ps []ModuleParams, existing []Module, newID func() ModuleID) []Module {
	known := make(map[ModuleID]ModuleKind, len(existing))
	for _, m := range existing {
		known[m.ID] = m.Kind
	}

	out := make([]Module, 0, len(ps))
	seenIDs := map[ModuleID]bool{}
	seenSingle := map[ModuleKind]bool{}
	seenPrimary := map[ModuleKind]bool{}

	for i, p := range ps {
		field := fmt.Sprintf("modules[%d]", i)

		m, ok := checkModule(f, field, p)
		if !ok {
			continue
		}

		switch {
		case p.ID == nil:
			m.ID = newID()
		case seenIDs[*p.ID] || known[*p.ID] != p.Kind:
			f.add(field+".id", FieldInvalid)
		default:
			m.ID = *p.ID
			seenIDs[m.ID] = true
		}

		if !p.Kind.Multi() {
			if seenSingle[p.Kind] {
				f.add(field+".kind", FieldInvalid)
			}

			seenSingle[p.Kind] = true
		}

		if p.Primary {
			if seenPrimary[p.Kind] {
				f.add(field+".primary", FieldInvalid)
			}

			seenPrimary[p.Kind] = true
		}

		out = append(out, m)
	}

	SortModules(out)

	return out
}

// checkModule validates the kind, primary flag and payload of one module;
// ok is false when the kind is unusable.
func checkModule(f fieldErrors, field string, p ModuleParams) (Module, bool) {
	switch {
	case p.Kind == "":
		f.add(field+".kind", FieldRequired)
		return Module{}, false
	case !p.Kind.Valid():
		f.add(field+".kind", FieldInvalid)
		return Module{}, false
	}

	m := Module{Kind: p.Kind, Primary: p.Primary}

	if p.Primary && !p.Kind.Multi() {
		f.add(field+".primary", FieldInvalid)
	}

	set := map[ModuleKind]bool{
		ModuleHardware: p.Hardware != nil, ModuleNetworkHost: p.NetworkHost != nil,
		ModulePurchase: p.Purchase != nil, ModuleSimCard: p.SimCard != nil,
		ModuleWWAN:             p.WWAN != nil,
		ModuleOperatingSystem:  p.OperatingSystem != nil,
		ModuleNetworkInterface: p.NetworkInterface != nil,
	}

	for kind, has := range set {
		if has && kind != p.Kind {
			f.add(field+"."+kind.payloadField(), FieldInvalid)
		}
	}

	at := field + "." + p.Kind.payloadField()
	if !set[p.Kind] {
		f.add(at, FieldRequired)
		return m, true
	}

	checkPayload(f, at, p, &m)

	return m, true
}

// checkPayload validates the payload of p's kind, which must be set, into m.
func checkPayload(f fieldErrors, at string, p ModuleParams, m *Module) {
	switch p.Kind {
	case ModuleHardware:
		m.Hardware = new(checkHardware(f, at, *p.Hardware))
	case ModuleNetworkHost:
		m.NetworkHost = new(checkNetworkHost(f, at, *p.NetworkHost))
	case ModulePurchase:
		m.Purchase = new(checkPurchase(f, at, *p.Purchase))
	case ModuleSimCard:
		m.SimCard = new(checkSimCard(f, at, *p.SimCard))
	case ModuleWWAN:
		m.WWAN = new(checkWWAN(f, at, *p.WWAN))
	case ModuleOperatingSystem:
		m.OperatingSystem = new(checkOperatingSystem(f, at, *p.OperatingSystem))
	case ModuleNetworkInterface:
		m.NetworkInterface = new(checkInterface(f, at, *p.NetworkInterface))
	}
}

// payloadField is the API field name of k's payload.
func (k ModuleKind) payloadField() string {
	switch k {
	case ModuleHardware:
		return "hardware"
	case ModuleNetworkHost:
		return "networkHost"
	case ModulePurchase:
		return "purchase"
	case ModuleSimCard:
		return "simCard"
	case ModuleWWAN:
		return "wwan"
	case ModuleOperatingSystem:
		return "operatingSystem"
	case ModuleNetworkInterface:
		return "networkInterface"
	}

	return string(k)
}

func equalPtr[T any](a, b *T, eq func(T, T) bool) bool {
	if a == nil || b == nil {
		return a == b
	}

	return eq(*a, *b)
}

// naturalCompare compares a and b ignoring case, treating runs of digits as
// numbers, so "eth2" sorts before "eth10".
func naturalCompare(a, b string) int {
	ar, br := []rune(strings.ToLower(a)), []rune(strings.ToLower(b))

	for len(ar) > 0 && len(br) > 0 {
		if unicode.IsDigit(ar[0]) && unicode.IsDigit(br[0]) {
			var an, bn []rune

			an, ar = splitDigits(ar)
			bn, br = splitDigits(br)

			if c := compareDigits(an, bn); c != 0 {
				return c
			}

			continue
		}

		if c := cmp.Compare(ar[0], br[0]); c != 0 {
			return c
		}

		ar, br = ar[1:], br[1:]
	}

	return cmp.Compare(len(ar), len(br))
}

func splitDigits(r []rune) (digits, rest []rune) {
	i := 0
	for i < len(r) && unicode.IsDigit(r[i]) {
		i++
	}

	return r[:i], r[i:]
}

// compareDigits compares two digit runs by numeric value.
func compareDigits(a, b []rune) int {
	trim := func(r []rune) []rune {
		for len(r) > 1 && r[0] == '0' {
			r = r[1:]
		}

		return r
	}

	a, b = trim(a), trim(b)
	if c := cmp.Compare(len(a), len(b)); c != 0 {
		return c
	}

	return slices.Compare(a, b)
}
