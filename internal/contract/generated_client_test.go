package contract_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	daikuv1 "github.com/DaikuFi/daiku-cli/generated/daikuv1"
)

func TestGeneratedClientDecodesExchangeRateFixture(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/v1/exchange-rates/" {
			http.Error(w, "unexpected request", http.StatusBadRequest)
			return
		}
		if got := r.Header.Get("Authorization"); got != "Bearer fixture-token" {
			http.Error(w, "missing bearer token", http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `[{"from_currency":"USD","to_currency":"UYU","rate":"40.125"}]`)
	}))
	t.Cleanup(server.Close)

	client, err := daikuv1.NewClientWithResponses(server.URL, daikuv1.WithRequestEditorFn(
		func(_ context.Context, req *http.Request) error {
			req.Header.Set("Authorization", "Bearer fixture-token")
			return nil
		},
	))
	if err != nil {
		t.Fatal(err)
	}

	response, err := client.DaikuExchangeRatesGetWithResponse(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode() != http.StatusOK || response.JSON200 == nil {
		t.Fatalf("response = status %d body %s", response.StatusCode(), response.Body)
	}
	rates := *response.JSON200
	if len(rates) != 1 || rates[0].Rate != "40.125" || rates[0].FromCurrency != "USD" {
		t.Fatalf("decoded rates = %#v", rates)
	}
}
