package domain

import (
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
)

// maxPriceCents bounds prices (below 10^12 currency units).
const maxPriceCents = 99_999_999_999_999

// pricePattern is a non-negative decimal with at most two decimals.
var pricePattern = regexp.MustCompile(`^[0-9]{1,12}(\.[0-9]{1,2})?$`)

// Purchase records how an item was bought: the payload of a ModulePurchase
// module. Every field is optional, but a price always has a currency.
type Purchase struct {
	Supplier      string
	InvoiceNumber string
	// PriceCents is the price in hundredths of the currency unit; nil
	// when unknown.
	PriceCents *int64
	// Currency is an upper-case ISO 4217 code; empty exactly when
	// PriceCents is nil.
	Currency    string
	WarrantyEnd *time.Time
}

// Equal reports whether p and o hold the same purchase details.
func (p Purchase) Equal(o Purchase) bool {
	return p.Supplier == o.Supplier && p.InvoiceNumber == o.InvoiceNumber && p.Currency == o.Currency &&
		equalPtr(p.PriceCents, o.PriceCents, func(a, b int64) bool { return a == b }) &&
		equalPtr(p.WarrantyEnd, o.WarrantyEnd, time.Time.Equal)
}

// String describes p in one line, for the audit log.
func (p Purchase) String() string {
	parts := []string{p.Supplier, p.InvoiceNumber}

	if p.PriceCents != nil {
		parts = append(parts, FormatPrice(*p.PriceCents)+" "+p.Currency)
	}

	if p.WarrantyEnd != nil {
		parts = append(parts, "warranty until "+p.WarrantyEnd.UTC().Format(time.RFC3339))
	}

	return strings.Join(parts, " / ")
}

// FormatPrice renders cents as a decimal with two places, e.g. "12.50".
func FormatPrice(cents int64) string {
	return fmt.Sprintf("%d.%02d", cents/100, cents%100)
}

// PurchaseParams is unvalidated input for a purchase module. Price is a
// decimal string such as "12.5"; empty means no price.
type PurchaseParams struct {
	Supplier      string
	InvoiceNumber string
	Price         string
	Currency      string
	WarrantyEnd   *time.Time
}

func checkPurchase(f fieldErrors, field string, p PurchaseParams) Purchase {
	out := Purchase{
		Supplier:      checkOptionalText(f, field+".supplier", p.Supplier, maxShortTextRunes),
		InvoiceNumber: checkOptionalText(f, field+".invoiceNumber", p.InvoiceNumber, maxShortTextRunes),
		WarrantyEnd:   p.WarrantyEnd,
	}

	price := strings.TrimSpace(p.Price)
	currency := strings.ToUpper(strings.TrimSpace(p.Currency))

	if price != "" {
		cents, ok := parsePrice(price)
		if ok {
			out.PriceCents = &cents
		} else {
			f.add(field+".price", FieldInvalid)
		}
	}

	switch {
	case currency == "" && price != "":
		f.add(field+".currency", FieldRequired)
	case currency == "":
	case price == "" || !validCurrency(currency):
		f.add(field+".currency", FieldInvalid)
	default:
		out.Currency = currency
	}

	return out
}

// parsePrice parses a non-negative decimal with at most two decimals into
// cents.
func parsePrice(s string) (int64, bool) {
	if !pricePattern.MatchString(s) {
		return 0, false
	}

	whole, frac, _ := strings.Cut(s, ".")
	frac = (frac + "00")[:2]

	cents, err := strconv.ParseInt(whole+frac, 10, 64)
	if err != nil || cents > maxPriceCents {
		return 0, false
	}

	return cents, true
}

func validCurrency(code string) bool {
	_, found := slices.BinarySearch(currencyCodes, code)

	return found
}

// currencyCodes are the active ISO 4217 currency codes, sorted.
var currencyCodes = []string{
	"AED", "AFN", "ALL", "AMD", "AOA", "ARS", "AUD", "AWG", "AZN", "BAM", "BBD", "BDT", "BGN", "BHD", "BIF",
	"BMD", "BND", "BOB", "BOV", "BRL", "BSD", "BTN", "BWP", "BYN", "BZD", "CAD", "CDF", "CHE", "CHF", "CHW",
	"CLF", "CLP", "CNY", "COP", "COU", "CRC", "CUP", "CVE", "CZK", "DJF", "DKK", "DOP", "DZD", "EGP", "ERN",
	"ETB", "EUR", "FJD", "FKP", "GBP", "GEL", "GHS", "GIP", "GMD", "GNF", "GTQ", "GYD", "HKD", "HNL", "HTG",
	"HUF", "IDR", "ILS", "INR", "IQD", "IRR", "ISK", "JMD", "JOD", "JPY", "KES", "KGS", "KHR", "KMF", "KPW",
	"KRW", "KWD", "KYD", "KZT", "LAK", "LBP", "LKR", "LRD", "LSL", "LYD", "MAD", "MDL", "MGA", "MKD", "MMK",
	"MNT", "MOP", "MRU", "MUR", "MVR", "MWK", "MXN", "MXV", "MYR", "MZN", "NAD", "NGN", "NIO", "NOK", "NPR",
	"NZD", "OMR", "PAB", "PEN", "PGK", "PHP", "PKR", "PLN", "PYG", "QAR", "RON", "RSD", "RUB", "RWF", "SAR",
	"SBD", "SCR", "SDG", "SEK", "SGD", "SHP", "SLE", "SOS", "SRD", "SSP", "STN", "SVC", "SYP", "SZL", "THB",
	"TJS", "TMT", "TND", "TOP", "TRY", "TTD", "TWD", "TZS", "UAH", "UGX", "USD", "USN", "UYI", "UYU", "UYW",
	"UZS", "VED", "VES", "VND", "VUV", "WST", "XAF", "XAG", "XAU", "XBA", "XBB", "XBC", "XBD", "XCD", "XCG",
	"XDR", "XOF", "XPD", "XPF", "XPT", "XSU", "XTS", "XUA", "XXX", "YER", "ZAR", "ZMW", "ZWG",
}
