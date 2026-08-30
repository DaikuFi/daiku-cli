package currency

import (
	"reflect"
	"testing"
)

func TestCodesMatchDaikuContract(t *testing.T) {
	want := []string{
		"UYU", "USD", "EUR", "BRL", "GBP", "ARS", "UI", "CLP", "COP", "MXN",
		"PEN", "PYG", "BOB", "VES", "GTQ", "HNL", "CRC", "NIO", "PAB", "DOP",
	}
	if !reflect.DeepEqual(Codes(), want) {
		t.Fatalf("Codes = %v, want %v", Codes(), want)
	}
}

func TestCodesReturnsACopy(t *testing.T) {
	result := Codes()
	result[0] = "BTC"
	if Codes()[0] != "UYU" {
		t.Fatalf("callers can mutate supported currencies: %v", Codes())
	}
}

func TestNormalizeAcceptsRegionalCurrencyAndRejectsUnknown(t *testing.T) {
	if got, ok := Normalize(" brl "); !ok || got != "BRL" {
		t.Fatalf("Normalize(BRL) = %q, %v", got, ok)
	}
	if got, ok := Normalize("BTC"); ok || got != "BTC" {
		t.Fatalf("Normalize(BTC) = %q, %v", got, ok)
	}
}
