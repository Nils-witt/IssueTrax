package domain

import "strings"

// Hardware identifies a physical device: the payload of a ModuleHardware
// module. Manufacturer and model are picked from the central list; all
// fields may be empty, but a model needs its manufacturer.
type Hardware struct {
	ManufacturerID *ManufacturerID
	ModelID        *HardwareModelID
	SerialNumber   string
}

// Equal reports whether h and o name the same device.
func (h Hardware) Equal(o Hardware) bool {
	return equalPtr(h.ManufacturerID, o.ManufacturerID, func(a, b ManufacturerID) bool { return a == b }) &&
		equalPtr(h.ModelID, o.ModelID, func(a, b HardwareModelID) bool { return a == b }) &&
		h.SerialNumber == o.SerialNumber
}

// String describes h in one line, for the audit log.
func (h Hardware) String() string {
	return strings.Join([]string{idOrEmpty(h.ManufacturerID), idOrEmpty(h.ModelID), h.SerialNumber}, " / ")
}

// HardwareParams is unvalidated input for a hardware module.
type HardwareParams struct {
	ManufacturerID *ManufacturerID
	ModelID        *HardwareModelID
	SerialNumber   string
}

// checkHardware validates the shape of p. Whether the manufacturer and
// model exist, are live and belong together needs storage.
func checkHardware(f fieldErrors, field string, p HardwareParams) Hardware {
	if p.ModelID != nil && p.ManufacturerID == nil {
		f.add(field+".manufacturerId", FieldRequired)
	}

	return Hardware{
		ManufacturerID: p.ManufacturerID,
		ModelID:        p.ModelID,
		SerialNumber:   checkOptionalText(f, field+".serialNumber", p.SerialNumber, maxShortTextRunes),
	}
}

func idOrEmpty[T interface{ String() string }](id *T) string {
	if id == nil {
		return ""
	}

	return (*id).String()
}

// HardwareOf returns the payload of the hardware module among modules, or
// nil when there is none.
func HardwareOf(modules []Module) *Hardware {
	for _, m := range modules {
		if m.Hardware != nil {
			return m.Hardware
		}
	}

	return nil
}
