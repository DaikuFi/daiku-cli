package api

import (
	"encoding/json"
	"testing"
)

func TestDecodePageVariants(t *testing.T) {
	t.Parallel()
	type item struct {
		Amount json.Number `json:"amount"`
	}
	for _, test := range []struct {
		name, input string
		count       *int
	}{{"bare array", `[{"amount":1.25}]`, nil}, {"drf results", `{"count":1,"next":"https://example.test/page=2","previous":null,"results":[{"amount":1.25}]}`, intPointer(1)}, {"data", `{"data":[{"amount":1.25}]}`, nil}, {"items", `{"items":[{"amount":1.25}]}`, nil}} {
		t.Run(test.name, func(t *testing.T) {
			page, err := DecodePage[item]([]byte(test.input))
			if err != nil {
				t.Fatal(err)
			}
			if len(page.Results) != 1 || page.Results[0].Amount.String() != "1.25" {
				t.Fatalf("page=%+v", page)
			}
			if test.count != nil && (page.Count == nil || *page.Count != *test.count) {
				t.Fatalf("count=%v", page.Count)
			}
		})
	}
}

func TestDecodePageRejectsMalformedShapes(t *testing.T) {
	t.Parallel()
	for _, input := range []string{`not-json`, `{}`, `{"results":{}}`, `[] []`, `[] trailing`} {
		if _, err := DecodePage[map[string]any]([]byte(input)); err == nil {
			t.Fatalf("accepted %s", input)
		}
	}
}

func intPointer(value int) *int { return &value }
