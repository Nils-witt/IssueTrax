package domain

import "strings"

// OperatingSystem is the operating system installed on a device: the payload
// of a ModuleOperatingSystem module. Both fields may be empty.
type OperatingSystem struct {
	Name    string
	Version string
}

// String describes o in one line, for the audit log.
func (o OperatingSystem) String() string {
	return strings.TrimSpace(o.Name + " " + o.Version)
}

// OperatingSystemParams is unvalidated input for an operating system module.
type OperatingSystemParams struct {
	Name    string
	Version string
}

func checkOperatingSystem(f fieldErrors, field string, p OperatingSystemParams) OperatingSystem {
	return OperatingSystem{
		Name:    checkOptionalText(f, field+".name", p.Name, maxShortTextRunes),
		Version: checkOptionalText(f, field+".version", p.Version, maxShortTextRunes),
	}
}
