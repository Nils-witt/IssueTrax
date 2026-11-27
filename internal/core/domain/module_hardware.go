package domain

import "strings"

// Hardware identifies a physical device: the payload of a ModuleHardware
// module. All fields may be empty.
type Hardware struct {
	Manufacturer string
	Model        string
	SerialNumber string
}

// String describes h in one line, for the audit log.
func (h Hardware) String() string {
	return strings.Join([]string{h.Manufacturer, h.Model, h.SerialNumber}, " / ")
}

// HardwareParams is unvalidated input for a hardware module.
type HardwareParams struct {
	Manufacturer string
	Model        string
	SerialNumber string
}

func checkHardware(f fieldErrors, field string, p HardwareParams) Hardware {
	return Hardware{
		Manufacturer: checkOptionalText(f, field+".manufacturer", p.Manufacturer, maxShortTextRunes),
		Model:        checkOptionalText(f, field+".model", p.Model, maxShortTextRunes),
		SerialNumber: checkOptionalText(f, field+".serialNumber", p.SerialNumber, maxShortTextRunes),
	}
}
