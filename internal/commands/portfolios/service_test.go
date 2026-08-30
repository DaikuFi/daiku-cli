package portfolios

import (
	"context"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/DaikuFi/daiku-cli/internal/profiles"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

type fakeTokens struct {
	calls int
	token string
}

func (f *fakeTokens) AccessToken(context.Context, string) (string, error) {
	f.calls++
	return f.token, nil
}

func TestGeneratedFactoryUsesTokenSourceAndPatchPreservesNullPayload(t *testing.T) {
	dir := t.TempDir()
	if err := os.Chmod(dir, 0700); err != nil {
		t.Fatal(err)
	}
	store := profiles.Store{Path: dir + "/profiles.json"}
	if err := store.Save(profiles.Config{Current: "personal", Profiles: map[string]profiles.Profile{"personal": {APIURL: "https://api.example.test/api/v1/"}}}); err != nil {
		t.Fatal(err)
	}
	tokens := &fakeTokens{token: "refreshed-access"}
	var body, authorization, path string
	httpClient := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		raw, _ := io.ReadAll(request.Body)
		body = string(raw)
		authorization = request.Header.Get("Authorization")
		path = request.URL.Path
		header := make(http.Header)
		header.Set("Content-Type", "application/json")
		return &http.Response{StatusCode: 200, Status: "200 OK", Header: header, Body: io.NopCloser(strings.NewReader(`{"id":"ast_1","name":"Asset","asset_type":"other","last_price_update":null,"linked_account":null,"price_per_unit":null,"quantity":null,"ticker_symbol":null}`)), Request: request}, nil
	})}
	service, err := GeneratedFactory(store, tokens, httpClient)(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.AssetUpdate(context.Background(), "bkt_1", "ast_1", map[string]any{"quantity": nil, "ticker_symbol": "ABC"}); err != nil {
		t.Fatal(err)
	}
	if tokens.calls != 1 || authorization != "Bearer refreshed-access" {
		t.Fatalf("auth calls=%d header=%q", tokens.calls, authorization)
	}
	if path != "/api/v1/buckets/bkt_1/assets/ast_1/" {
		t.Fatalf("path=%s", path)
	}
	if body != `{"quantity":null,"ticker_symbol":"ABC"}` {
		t.Fatalf("payload=%s", body)
	}
}
