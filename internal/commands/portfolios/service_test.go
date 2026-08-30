package portfolios

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/DaikuFi/daiku-cli/internal/cli"
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

func executeWithGeneratedFactory(t *testing.T, server *httptest.Server, args ...string) (int, string, string) {
	t.Helper()
	dir := t.TempDir()
	if err := os.Chmod(dir, 0700); err != nil {
		t.Fatal(err)
	}
	store := profiles.Store{Path: dir + "/profiles.json"}
	if err := store.Save(profiles.Config{Current: "personal", Profiles: map[string]profiles.Profile{"personal": {APIURL: server.URL + "/api/v1/"}}}); err != nil {
		t.Fatal(err)
	}
	tokens := &fakeTokens{token: "refreshed-access"}
	var stdout, stderr bytes.Buffer
	module := New(GeneratedFactory(store, tokens, server.Client()))
	app := cli.New(
		cli.WithIO(strings.NewReader(""), &stdout, &stderr),
		cli.WithModule(module),
		cli.WithEnvironment(func(string) (string, bool) { return "", false }),
	)
	code := app.Run(args)
	if tokens.calls != 1 {
		t.Fatalf("token calls=%d", tokens.calls)
	}
	return code, stdout.String(), stderr.String()
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

func TestCashflowListHTTPPreservesLinkedTransactionState(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet || request.URL.Path != "/api/v1/assets/ast_1/cashflows/" {
			t.Errorf("request=%s %s", request.Method, request.URL.Path)
			writer.WriteHeader(http.StatusNotFound)
			return
		}
		if authorization := request.Header.Get("Authorization"); authorization != "Bearer refreshed-access" {
			t.Errorf("authorization=%q", authorization)
			writer.WriteHeader(http.StatusUnauthorized)
			return
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(writer, `[{
			"id":"cf_1","asset":"ast_1","date":"2026-08-30",
			"cash_in":"10.00","cash_in_currency":"USD","cash_in_converted":"10.00",
			"cash_out":"4.00","cash_out_currency":"UYU","cash_out_converted":"0.10",
			"transaction_links":{
				"cash_in":{"id":"lnk_in","side":"cash_in","visibility":"visible","transaction":{"id":"exp_in","description":"Deposit","amount":"10.00","currency":"USD","expense_date":"2026-08-30","is_income":true,"household":"hsh_1","account_name":null}},
				"cash_out":{"id":"lnk_out","side":"cash_out","visibility":"visible","transaction":{"id":"exp_out","description":"Fee","amount":"4.00","currency":"UYU","expense_date":"2026-08-30","is_income":false,"household":"hsh_1","account_name":"Checking"}}
			}
		}]`)
	}))
	defer server.Close()

	code, stdout, stderr := executeWithGeneratedFactory(t, server, "assets", "cashflows", "list", "--asset", "ast_1", "--json")
	if code != int(cli.ExitOK) || stderr != "" {
		t.Fatalf("code=%d stderr=%s", code, stderr)
	}
	for _, expected := range []string{
		`"cash_in":{"id":"lnk_in","side":"cash_in"`,
		`"id":"exp_in"`,
		`"cash_out":{"id":"lnk_out","side":"cash_out"`,
		`"id":"exp_out"`,
	} {
		if !strings.Contains(stdout, expected) {
			t.Errorf("missing %s in %s", expected, stdout)
		}
	}
}

func TestCashflowListHTTPPreservesUnlinkedStates(t *testing.T) {
	for _, test := range []struct {
		name  string
		links string
	}{
		{name: "null", links: "null"},
		{name: "empty object", links: "{}"},
	} {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				if request.Method != http.MethodGet || request.URL.Path != "/api/v1/assets/ast_1/cashflows/" {
					t.Errorf("request=%s %s", request.Method, request.URL.Path)
					writer.WriteHeader(http.StatusNotFound)
					return
				}
				if authorization := request.Header.Get("Authorization"); authorization != "Bearer refreshed-access" {
					t.Errorf("authorization=%q", authorization)
					writer.WriteHeader(http.StatusUnauthorized)
					return
				}
				writer.Header().Set("Content-Type", "application/json")
				_, _ = io.WriteString(writer, `[{"id":"cf_1","asset":"ast_1","date":"2026-08-30","cash_in":null,"cash_in_currency":null,"cash_in_converted":null,"cash_out":null,"cash_out_currency":null,"cash_out_converted":null,"transaction_links":`+test.links+`}]`)
			}))
			defer server.Close()

			code, stdout, stderr := executeWithGeneratedFactory(t, server, "assets", "cashflows", "list", "--asset", "ast_1", "--json")
			if code != int(cli.ExitOK) || stderr != "" {
				t.Fatalf("code=%d stderr=%s", code, stderr)
			}
			if strings.Contains(stdout, `"transaction":{"`) || strings.Contains(stdout, "lnk_") {
				t.Fatalf("unlinked state gained transaction data: %s", stdout)
			}
			if test.links == "{}" && !strings.Contains(stdout, `"transaction_links":{}`) {
				t.Fatalf("empty linked state was not preserved: %s", stdout)
			}
		})
	}
}

func TestCashflowListHTTPMapsForeignAssetWithoutLeakage(t *testing.T) {
	for _, test := range []struct {
		status int
		exit   cli.ExitCode
		code   string
	}{
		{status: http.StatusForbidden, exit: cli.ExitForbidden, code: "forbidden"},
		{status: http.StatusNotFound, exit: cli.ExitNotFound, code: "not_found"},
	} {
		t.Run(http.StatusText(test.status), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				if request.Method != http.MethodGet || request.URL.Path != "/api/v1/assets/ast_foreign/cashflows/" {
					t.Errorf("request=%s %s", request.Method, request.URL.Path)
					writer.WriteHeader(http.StatusNotFound)
					return
				}
				if authorization := request.Header.Get("Authorization"); authorization != "Bearer refreshed-access" {
					t.Errorf("authorization=%q", authorization)
					writer.WriteHeader(http.StatusUnauthorized)
					return
				}
				writer.Header().Set("Content-Type", "application/json")
				writer.WriteHeader(test.status)
				_, _ = io.WriteString(writer, `{"error":{"status_code":`+strconv.Itoa(test.status)+`,"message":"denied","errors":null},"owner_email":"victim@example.test","internal_asset":"ast_foreign"}`)
			}))
			defer server.Close()

			code, stdout, stderr := executeWithGeneratedFactory(t, server, "assets", "cashflows", "list", "--asset", "ast_foreign", "--json")
			if code != int(test.exit) || stdout != "" || !strings.Contains(stderr, `"code":"`+test.code+`"`) {
				t.Fatalf("code=%d stdout=%s stderr=%s", code, stdout, stderr)
			}
			for _, secret := range []string{"victim@example.test", "ast_foreign", "owner_email", "internal_asset"} {
				if strings.Contains(stderr, secret) {
					t.Fatalf("response leaked %q: %s", secret, stderr)
				}
			}
		})
	}
}
