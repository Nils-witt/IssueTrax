package domain

import (
	"regexp"
	"strings"
)

var (
	// phoneNumberPattern is an optional "+" and 3 to 15 digits (E.164 allows
	// at most 15), after separators are removed.
	phoneNumberPattern = regexp.MustCompile(`^\+?[0-9]{3,15}$`)
	// iccidPattern is an ICCID (ITU-T E.118): 18 to 22 digits starting with
	// the telecom industry prefix 89. The Luhn check digit is not verified
	// because cards print it inconsistently.
	iccidPattern = regexp.MustCompile(`^89[0-9]{16,20}$`)
	// numberSeparators are removed from phone numbers and ICCIDs.
	numberSeparators = strings.NewReplacer(" ", "", "-", "", "/", "", "(", "", ")", "", ".", "")
)

// SimCard is a SIM card placed in a device: the payload of a ModuleSimCard
// module. Every field may be empty. PhoneNumber and ICCID are kept without
// separators.
type SimCard struct {
	Provider    string
	PhoneNumber string
	ICCID       string
}

// String describes c in one line, for the audit log.
func (c SimCard) String() string {
	return strings.Join([]string{c.Provider, c.PhoneNumber, c.ICCID}, " / ")
}

// SimCardParams is unvalidated input for a SIM card module.
type SimCardParams struct {
	Provider    string
	PhoneNumber string
	ICCID       string
}

func checkSimCard(f fieldErrors, field string, p SimCardParams) SimCard {
	return SimCard{
		Provider:    checkOptionalText(f, field+".provider", p.Provider, maxShortTextRunes),
		PhoneNumber: checkNumber(f, field+".phoneNumber", p.PhoneNumber, phoneNumberPattern),
		ICCID:       checkNumber(f, field+".iccid", p.ICCID, iccidPattern),
	}
}

// checkNumber removes separators from s and checks it is empty or matches
// pattern.
func checkNumber(f fieldErrors, field, s string, pattern *regexp.Regexp) string {
	s = numberSeparators.Replace(strings.TrimSpace(s))
	if s != "" && !pattern.MatchString(s) {
		f.add(field, FieldInvalid)
	}

	return s
}
