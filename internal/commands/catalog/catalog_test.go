package catalog_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/DaikuFi/daiku-cli/internal/cli"
	"github.com/DaikuFi/daiku-cli/internal/commands/catalog"
	"github.com/DaikuFi/daiku-cli/internal/profiles"
)

type tokens struct{}

func (tokens) AccessToken(context.Context, string) (string, error) { return "fixture-token", nil }

func testApp(t *testing.T, handler http.HandlerFunc, input string, interactive bool) (*cli.App, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	store := profiles.Store{Path: filepath.Join(dir, "config.json")}
	if err := store.Save(profiles.Config{Current: "test", Profiles: map[string]profiles.Profile{"test": {APIURL: "https://fixture.invalid/api/v1/"}}}); err != nil {
		t.Fatal(err)
	}
	out, errOut := new(bytes.Buffer), new(bytes.Buffer)
	httpClient := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		recorder := newResponseRecorder()
		handler.ServeHTTP(recorder, r)
		return recorder.response(r), nil
	})}
	app := cli.New(cli.WithIO(strings.NewReader(input), out, errOut), cli.WithInteractiveDetector(func(_ io.Reader, _ io.Writer) bool { return interactive }), cli.WithTerminalDetector(func(io.Writer) bool { return false }), cli.WithModule(catalog.New(store, tokens{}, httpClient)))
	return app, out, errOut
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

type responseRecorder struct {
	header http.Header
	body   bytes.Buffer
	status int
}

func newResponseRecorder() *responseRecorder       { return &responseRecorder{header: make(http.Header)} }
func (r *responseRecorder) Header() http.Header    { return r.header }
func (r *responseRecorder) WriteHeader(status int) { r.status = status }
func (r *responseRecorder) Write(data []byte) (int, error) {
	if r.status == 0 {
		r.status = http.StatusOK
	}
	return r.body.Write(data)
}
func (r *responseRecorder) response(req *http.Request) *http.Response {
	if r.status == 0 {
		r.status = http.StatusOK
	}
	return &http.Response{StatusCode: r.status, Header: r.header, Body: io.NopCloser(bytes.NewReader(r.body.Bytes())), Request: req}
}

func apiHandler(t *testing.T, method, path string, status int, response string, check func(*http.Request)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		t.Helper()
		if r.Method != method || r.URL.RequestURI() != path {
			t.Errorf("request = %s %s, want %s %s", r.Method, r.URL.RequestURI(), method, path)
		}
		if r.Header.Get("Authorization") != "Bearer fixture-token" {
			t.Errorf("authorization missing")
		}
		if check != nil {
			check(r)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(response))
	}
}

func TestHouseholdsListJSONIsDeterministic(t *testing.T) {
	app, out, _ := testApp(t, apiHandler(t, "GET", "/api/v1/households/", 200, `[{"id":"hh_2","name":"Zulu"},{"id":"hh_1","name":"Alpha"}]`, nil), "", false)
	if code := app.Run([]string{"households", "list", "--json"}); code != 0 {
		t.Fatalf("exit=%d", code)
	}
	want := "{\"ok\":true,\"data\":{\"households\":[{\"id\":\"hh_1\",\"name\":\"Alpha\"},{\"id\":\"hh_2\",\"name\":\"Zulu\"}]}}\n"
	if out.String() != want {
		t.Fatalf("output=%q", out.String())
	}
}

func TestSpanishHumanHelpKeepsCommandsEnglish(t *testing.T) {
	app, out, _ := testApp(t, func(http.ResponseWriter, *http.Request) { t.Fatal("network called") }, "", true)
	if code := app.Run([]string{"households", "--help", "--language", "es"}); code != 0 {
		t.Fatalf("exit=%d", code)
	}
	if !strings.Contains(out.String(), "Gestiona hogares") || !strings.Contains(out.String(), "create") {
		t.Fatal(out.String())
	}
}

func TestScopedCommandRequiresHouseholdWithoutNetwork(t *testing.T) {
	called := false
	app, _, errOut := testApp(t, func(http.ResponseWriter, *http.Request) { called = true }, "", false)
	if code := app.Run([]string{"accounts", "list", "--json"}); code != 2 {
		t.Fatalf("exit=%d stderr=%s", code, errOut.String())
	}
	if called {
		t.Fatal("network called")
	}
	if !strings.Contains(errOut.String(), `"code":"usage_error"`) {
		t.Fatal(errOut.String())
	}
}

func TestNameAmbiguityReturnsStableCandidates(t *testing.T) {
	h := apiHandler(t, "GET", "/api/v1/households/hh_1/tags/", 200, `[{"id":"tag_b","name":"Trip"},{"id":"tag_a","name":"trip"}]`, nil)
	app, _, errOut := testApp(t, h, "", false)
	if code := app.Run([]string{"tags", "delete", "Trip", "--household", "hh_1", "--yes", "--json"}); code != 6 {
		t.Fatalf("exit=%d stderr=%s", code, errOut.String())
	}
	var envelope map[string]any
	if json.Unmarshal(errOut.Bytes(), &envelope) != nil {
		t.Fatal(errOut.String())
	}
	if !strings.Contains(errOut.String(), `"code":"ambiguous_resource"`) {
		t.Fatal(errOut.String())
	}
}

func TestViewerWriteIsServerAuthorized(t *testing.T) {
	h := apiHandler(t, "POST", "/api/v1/households/hh_1/tags/", 403, `{"error":{"message":"forbidden"}}`, nil)
	app, _, errOut := testApp(t, h, "", false)
	if code := app.Run([]string{"tags", "create", "--household", "hh_1", "--name", "Tax", "--json"}); code != 4 {
		t.Fatalf("exit=%d stderr=%s", code, errOut.String())
	}
	if !strings.Contains(errOut.String(), `"code":"forbidden"`) {
		t.Fatal(errOut.String())
	}
}

func TestDestructiveCommandNeedsYesWhenNonInteractive(t *testing.T) {
	h := apiHandler(t, "GET", "/api/v1/households/hh_1/accounts/?archived=all", 200, `[{"id":"acc_1","name":"Cash"}]`, nil)
	app, _, errOut := testApp(t, h, "", false)
	if code := app.Run([]string{"accounts", "archive", "Cash", "--household", "hh_1", "--json"}); code != 2 {
		t.Fatalf("exit=%d stderr=%s", code, errOut.String())
	}
	if !strings.Contains(errOut.String(), `"code":"confirmation_required"`) {
		t.Fatal(errOut.String())
	}
}

func TestAccountAdjustUsesGeneratedContractShape(t *testing.T) {
	calls := 0
	h := func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		switch calls {
		case 1:
			if r.Method != "GET" {
				t.Fatal(r.Method)
			}
			_, _ = w.Write([]byte(`[{"id":"acc_1","name":"Cash"}]`))
		case 2:
			if r.Method != "POST" || r.URL.Path != "/api/v1/households/hh_1/accounts/acc_1/adjust/" {
				t.Fatalf("%s %s", r.Method, r.URL.Path)
			}
			var body map[string]any
			if json.NewDecoder(r.Body).Decode(&body) != nil || body["target_balance"] != "125.50" {
				t.Fatalf("body=%v", body)
			}
			_, _ = w.Write([]byte(`{"id":"exp_1","target_balance":"125.50"}`))
		}
	}
	app, out, errOut := testApp(t, h, "", false)
	if code := app.Run([]string{"accounts", "adjust", "Cash", "--household", "hh_1", "--target-balance", "125.50", "--yes", "--json"}); code != 0 {
		t.Fatalf("exit=%d stderr=%s", code, errOut.String())
	}
	if !strings.Contains(out.String(), `"target_balance":"125.50"`) {
		t.Fatal(out.String())
	}
}
