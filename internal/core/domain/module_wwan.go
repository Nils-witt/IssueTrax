package domain

import (
	"regexp"
	"strings"
)

// imeiPattern is an IMEI (3GPP TS 23.003): 15 digits, the last a Luhn check
// digit, after separators are removed.
var imeiPattern = regexp.MustCompile(`^[0-9]{15}$`)

// WWANTechnology is the mobile network generation a modem connects with.
type WWANTechnology string

// WWAN technologies. The empty technology means unknown.
const (
	WWAN2G WWANTechnology = "2g"
	WWAN3G WWANTechnology = "3g"
	WWAN4G WWANTechnology = "4g"
	WWAN5G WWANTechnology = "5g"
)

// Valid reports whether t is a known technology.
func (t WWANTechnology) Valid() bool {
	return t == WWAN2G || t == WWAN3G || t == WWAN4G || t == WWAN5G
}

// WWAN is a device's mobile (cellular) connection: the payload of a ModuleWWAN
// module. Every field may be empty. IMEI is kept without separators.
type WWAN struct {
	IMEI       string
	APN        string
	Technology WWANTechnology
}

// String describes w in one line, for the audit log.
func (w WWAN) String() string {
	return strings.Join([]string{w.IMEI, w.APN, string(w.Technology)}, " / ")
}

// WWANParams is unvalidated input for a WWAN module.
type WWANParams struct {
	IMEI       string
	APN        string
	Technology WWANTechnology
}

func checkWWAN(f fieldErrors, field string, p WWANParams) WWAN {
	imei := checkNumber(f, field+".imei", p.IMEI, imeiPattern)
	if imei != "" && imeiPattern.MatchString(imei) && !luhnValid(imei) {
		f.add(field+".imei", FieldInvalid)
	}

	if p.Technology != "" && !p.Technology.Valid() {
		f.add(field+".technology", FieldInvalid)
	}

	return WWAN{
		IMEI:       imei,
		APN:        checkOptionalText(f, field+".apn", p.APN, maxShortTextRunes),
		Technology: p.Technology,
	}
}

// luhnValid reports whether the digit string s passes the Luhn check.
func luhnValid(s string) bool {
	sum := 0

	for i := range len(s) {
		d := int(s[len(s)-1-i] - '0')
		if i%2 == 1 {
			d *= 2
			if d > 9 {
				d -= 9
			}
		}

		sum += d
	}

	return sum%10 == 0
}
