// Package currency defines the currency codes accepted by Daiku's public API.
package currency

import "strings"

var codes = [...]string{
	"UYU", "USD", "EUR", "BRL", "GBP", "ARS", "UI", "CLP", "COP", "MXN",
	"PEN", "PYG", "BOB", "VES", "GTQ", "HNL", "CRC", "NIO", "PAB", "DOP",
}

var supported = func() map[string]struct{} {
	result := make(map[string]struct{}, len(codes))
	for _, code := range codes {
		result[code] = struct{}{}
	}
	return result
}()

// Codes returns the ordered set of currencies supported by Daiku.
func Codes() []string {
	result := make([]string, len(codes))
	copy(result, codes[:])
	return result
}

// Normalize trims and uppercases a currency code, returning whether Daiku
// supports it.
func Normalize(value string) (string, bool) {
	value = strings.ToUpper(strings.TrimSpace(value))
	_, ok := supported[value]
	return value, ok
}

// IsSupported reports whether value is an exact, normalized Daiku currency
// code.
func IsSupported(value string) bool {
	_, ok := supported[value]
	return ok
}
