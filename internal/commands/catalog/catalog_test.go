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

func TestNameContainingUnderscoreIsResolvedInsteadOfTrustedAsID(t *testing.T) {
	calls := 0
	h := func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		if calls == 1 {
			if r.Method != "GET" {
				t.Fatal(r.Method)
			}
			_, _ = w.Write([]byte(`[{"id":"tag_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","name":"summer_trip"}]`))
			return
		}
		if r.Method != "DELETE" || !strings.HasSuffix(r.URL.Path, "tag_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa/") {
			t.Fatalf("%s %s", r.Method, r.URL.Path)
		}
		w.WriteHeader(204)
	}
	app, _, errOut := testApp(t, h, "", false)
	if code := app.Run([]string{"tags", "delete", "summer_trip", "--household", "hh_1", "--yes", "--json"}); code != 0 {
		t.Fatalf("exit=%d stderr=%s", code, errOut.String())
	}
	if calls != 2 {
		t.Fatalf("calls=%d", calls)
	}
}

func TestWrongResourceIDPrefixFailsBeforeNetwork(t *testing.T) {
	called := false
	app, _, errOut := testApp(t, func(http.ResponseWriter, *http.Request) { called = true }, "", false)
	if code := app.Run([]string{"tags", "delete", "acc_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "--household", "hh_1", "--yes", "--json"}); code != 2 {
		t.Fatalf("exit=%d %s", code, errOut.String())
	}
	if called {
		t.Fatal("network called")
	}
}

func TestInapplicableResourceFlagIsRejected(t *testing.T) {
	called := false
	app, _, errOut := testApp(t, func(http.ResponseWriter, *http.Request) { called = true }, "", false)
	if code := app.Run([]string{"tags", "create", "--household", "hh_1", "--name", "x", "--emoji", "x", "--json"}); code != 2 {
		t.Fatalf("exit=%d %s", code, errOut.String())
	}
	if called {
		t.Fatal("network called")
	}
}

func TestAccountPatchSupportsChangedFalseAndNullableFields(t *testing.T) {
	id := "acc_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	h := apiHandler(t, "PATCH", "/api/v1/households/hh_1/accounts/"+id+"/", 200, `{"id":"`+id+`","name":"Cash"}`, func(r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body["group"] != nil || body["institution"] != nil || body["is_default"] != false {
			t.Fatalf("body=%v", body)
		}
	})
	app, _, errOut := testApp(t, h, "", false)
	if code := app.Run([]string{"accounts", "update", id, "--household", "hh_1", "--clear-group", "--clear-institution", "--is-default=false", "--json"}); code != 0 {
		t.Fatalf("exit=%d %s", code, errOut.String())
	}
}

func TestSpanishDestructivePromptIsLocalized(t *testing.T) {
	id := "acc_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	calls := 0
	h := func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		if r.Method != "DELETE" {
			t.Fatalf("method=%s", r.Method)
		}
		_, _ = w.Write([]byte(`{"archived":true}`))
	}
	app, _, errOut := testApp(t, h, "sí\n", true)
	if code := app.Run([]string{"accounts", "archive", id, "--household", "hh_1", "--language", "es"}); code != 0 {
		t.Fatalf("exit=%d stderr=%s", code, errOut.String())
	}
	if calls != 1 || !strings.Contains(errOut.String(), "Archivar la cuenta") || !strings.Contains(errOut.String(), "¿Continuar?") {
		t.Fatalf("calls=%d prompt=%s", calls, errOut.String())
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

func TestCategoryReorderUsesGeneratedArrayBody(t *testing.T) {
	h := apiHandler(t, "POST", "/api/v1/households/hh_1/categories/reorder/", 200, `[]`, func(r *http.Request) {
		var body []map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if len(body) != 2 || body[0]["id"] != "cat_a" || body[0]["sort_order"] != float64(0) || body[1]["id"] != "cat_b" {
			t.Fatalf("body=%v", body)
		}
	})
	app, _, errOut := testApp(t, h, "", false)
	if code := app.Run([]string{"categories", "reorder", "--household", "hh_1", "--id", "cat_a,cat_b", "--json"}); code != 0 {
		t.Fatalf("exit=%d stderr=%s", code, errOut.String())
	}
}

func TestCatalogHTTPMatrix(t *testing.T) {
	hsh := "hsh_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	agp := "agp_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	acc := "acc_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	cat := "cat_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	tag := "tag_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	inst := "inst_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	tests := []struct {
		name, method, path string
		args               []string
		body               string
		status             int
	}{
		{"household create", "POST", "/api/v1/households/", []string{"households", "create", "--name", "Home", "--display-currency", "USD"}, `"name":"Home"`, 201},
		{"household get", "GET", "/api/v1/households/" + hsh + "/", []string{"households", "get", hsh}, "", 200},
		{"household update", "PATCH", "/api/v1/households/" + hsh + "/", []string{"households", "update", hsh, "--name", "Casa"}, `"name":"Casa"`, 200},
		{"household delete", "DELETE", "/api/v1/households/" + hsh + "/", []string{"households", "delete", hsh, "--yes"}, "", 204},
		{"household reorder", "POST", "/api/v1/households/reorder/", []string{"households", "reorder", "--id", hsh}, `"id":"` + hsh + `"`, 200},
		{"group create", "POST", "/api/v1/households/" + hsh + "/account-groups/", []string{"account-groups", "create", "--household", hsh, "--name", "Cash"}, `"name":"Cash"`, 201},
		{"group update", "PATCH", "/api/v1/households/" + hsh + "/account-groups/" + agp + "/", []string{"account-groups", "update", agp, "--household", hsh, "--emoji", "💵"}, `"emoji":"💵"`, 200},
		{"group delete", "DELETE", "/api/v1/households/" + hsh + "/account-groups/" + agp + "/", []string{"account-groups", "delete", agp, "--household", hsh, "--yes"}, "", 204},
		{"group reorder", "POST", "/api/v1/households/" + hsh + "/account-groups/reorder/", []string{"account-groups", "reorder", "--household", hsh, "--id", agp}, `"id":"` + agp + `"`, 200},
		{"account create", "POST", "/api/v1/households/" + hsh + "/accounts/", []string{"accounts", "create", "--household", hsh, "--name", "Bank", "--currency", "USD", "--is-default"}, `"is_default":true`, 201},
		{"account update", "PATCH", "/api/v1/households/" + hsh + "/accounts/" + acc + "/", []string{"accounts", "update", acc, "--household", hsh, "--currency", "UYU", "--account-holder", "Fran"}, `"account_holder":"Fran"`, 200},
		{"account archive", "DELETE", "/api/v1/households/" + hsh + "/accounts/" + acc + "/", []string{"accounts", "archive", acc, "--household", hsh, "--yes"}, "", 200},
		{"account unarchive", "POST", "/api/v1/households/" + hsh + "/accounts/" + acc + "/unarchive/", []string{"accounts", "unarchive", acc, "--household", hsh}, "", 200},
		{"account adjust", "POST", "/api/v1/households/" + hsh + "/accounts/" + acc + "/adjust/", []string{"accounts", "adjust", acc, "--household", hsh, "--target-balance", "9", "--yes"}, `"target_balance":"9"`, 200},
		{"account reorder", "POST", "/api/v1/households/" + hsh + "/accounts/reorder/", []string{"accounts", "reorder", "--household", hsh, "--id", acc}, `"id":"` + acc + `"`, 200},
		{"category create", "POST", "/api/v1/households/" + hsh + "/categories/", []string{"categories", "create", "--household", hsh, "--name", "Food", "--parent", cat}, `"parent":"` + cat + `"`, 201},
		{"category update", "PATCH", "/api/v1/households/" + hsh + "/categories/" + cat + "/", []string{"categories", "update", cat, "--household", hsh, "--clear-parent"}, `"parent":null`, 200},
		{"category delete", "DELETE", "/api/v1/households/" + hsh + "/categories/" + cat + "/", []string{"categories", "delete", cat, "--household", hsh, "--yes"}, "", 204},
		{"category reorder", "POST", "/api/v1/households/" + hsh + "/categories/reorder/", []string{"categories", "reorder", "--household", hsh, "--id", cat}, `"id":"` + cat + `"`, 200},
		{"tag create", "POST", "/api/v1/households/" + hsh + "/tags/", []string{"tags", "create", "--household", hsh, "--name", "Tax", "--color", "#fff"}, `"color":"#fff"`, 201},
		{"tag update", "PATCH", "/api/v1/households/" + hsh + "/tags/" + tag + "/", []string{"tags", "update", tag, "--household", hsh, "--name", "Taxes"}, `"name":"Taxes"`, 200},
		{"tag delete", "DELETE", "/api/v1/households/" + hsh + "/tags/" + tag + "/", []string{"tags", "delete", tag, "--household", hsh, "--yes"}, "", 204},
		{"institution create", "POST", "/api/v1/households/" + hsh + "/institutions/", []string{"institutions", "create", "--household", hsh, "--name", "Bank", "--country", "UY"}, `"country":"UY"`, 201},
		{"institution update", "PATCH", "/api/v1/households/" + hsh + "/institutions/" + inst + "/", []string{"institutions", "update", inst, "--household", hsh, "--domain", "bank.test"}, `"domain":"bank.test"`, 200},
		{"institution delete", "DELETE", "/api/v1/households/" + hsh + "/institutions/" + inst + "/", []string{"institutions", "delete", inst, "--household", hsh, "--yes"}, "", 204},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := apiHandler(t, tc.method, tc.path, tc.status, func() string {
				if tc.status == 204 {
					return ""
				}
				return `{}`
			}(), func(r *http.Request) {
				if tc.body == "" {
					return
				}
				raw, err := io.ReadAll(r.Body)
				if err != nil {
					t.Fatal(err)
				}
				if !strings.Contains(string(raw), tc.body) {
					t.Fatalf("body=%s want fragment=%s", raw, tc.body)
				}
			})
			app, _, errOut := testApp(t, h, "", false)
			args := append(append([]string{}, tc.args...), "--json")
			if code := app.Run(args); code != 0 {
				t.Fatalf("exit=%d stderr=%s", code, errOut.String())
			}
		})
	}
}

func TestCatalogMapsHTTPStatusesToStableExitCodes(t *testing.T) {
	id := "hsh_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	for _, tc := range []struct {
		status, exit int
		code         string
	}{{401, 3, "authentication_required"}, {403, 4, "forbidden"}, {404, 5, "not_found"}, {409, 6, "conflict"}, {429, 7, "rate_limited"}, {503, 7, "api_unavailable"}} {
		t.Run(tc.code, func(t *testing.T) {
			app, _, errOut := testApp(t, apiHandler(t, "GET", "/api/v1/households/"+id+"/", tc.status, `{"error":{}}`, nil), "", false)
			if code := app.Run([]string{"households", "get", id, "--json"}); code != tc.exit {
				t.Fatalf("exit=%d stderr=%s", code, errOut.String())
			}
			if !strings.Contains(errOut.String(), `"code":"`+tc.code+`"`) {
				t.Fatal(errOut.String())
			}
		})
	}
}
