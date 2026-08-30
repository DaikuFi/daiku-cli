package recurring

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	daikuv1 "github.com/DaikuFi/daiku-cli/generated/daikuv1"
	authcore "github.com/DaikuFi/daiku-cli/internal/auth"
	"github.com/DaikuFi/daiku-cli/internal/cli"
	"github.com/DaikuFi/daiku-cli/internal/credentials"
	"github.com/DaikuFi/daiku-cli/internal/profiles"
)

type memoryTokenStore struct {
	token credentials.Token
	puts  int
}

func (s *memoryTokenStore) Get(string) (credentials.Token, error) { return s.token, nil }
func (s *memoryTokenStore) Put(_ string, token credentials.Token) error {
	s.token = token
	s.puts++
	return nil
}
func (s *memoryTokenStore) Delete(string) error { return nil }

func testRecurringAPI(t *testing.T, handler http.HandlerFunc) generatedAPI {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	client, err := daikuv1.NewClientWithResponses(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	return generatedAPI{client}
}

func TestNewRefreshesAuthenticationBeforeBuildingAPI(t *testing.T) {
	var refreshForm url.Values
	var authorization string
	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/households/hh_1/recurring/" {
			t.Fatalf("API path=%q", r.URL.Path)
		}
		authorization = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `[]`)
	}))
	defer apiServer.Close()
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/oauth/token/" {
			t.Fatalf("path=%q", r.URL.Path)
		}
		if err := r.ParseForm(); err != nil {
			t.Fatal(err)
		}
		refreshForm = r.Form
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"access_token":"fresh","refresh_token":"rotated","expires_in":3600,"scope":"finance:read"}`)
	}))
	defer server.Close()
	oauth, err := authcore.New(authcore.Config{ClientID: "daiku-cli", AuthorizeURL: server.URL + "/oauth/authorize/", TokenURL: server.URL + "/oauth/token/", RevokeURL: server.URL + "/oauth/revoke/", HTTPClient: server.Client()})
	if err != nil {
		t.Fatal(err)
	}
	profileDir := t.TempDir()
	if err = os.Chmod(profileDir, 0o700); err != nil {
		t.Fatal(err)
	}
	profileStore := profiles.Store{Path: filepath.Join(profileDir, "profiles.json")}
	if err = profileStore.Save(profiles.Config{Current: "work", Profiles: map[string]profiles.Profile{"work": {APIURL: apiServer.URL + "/api/v1/"}}}); err != nil {
		t.Fatal(err)
	}
	tokens := &memoryTokenStore{token: credentials.Token{AccessToken: "expired", RefreshToken: "refresh-secret", ExpiresAt: time.Now().Add(-time.Minute).Unix()}}
	manager := &authcore.Manager{Store: tokens, OAuth: oauth, DisableProcessLock: true}
	api, err := New(profileStore, manager).Factory(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if _, err = api.List(context.Background(), "hh_1"); err != nil {
		t.Fatal(err)
	}
	if refreshForm.Get("grant_type") != "refresh_token" || refreshForm.Get("refresh_token") != "refresh-secret" || tokens.puts != 1 || tokens.token.AccessToken != "fresh" {
		t.Fatalf("form=%v puts=%d token=%+v", refreshForm, tokens.puts, tokens.token)
	}
	if authorization != "Bearer fresh" {
		t.Fatalf("authorization=%q", authorization)
	}
}

func TestGeneratedRecurringAPIUsesExactURLAndPatchNullSemantics(t *testing.T) {
	var requestLine string
	var patch map[string]any
	api := testRecurringAPI(t, func(w http.ResponseWriter, r *http.Request) {
		requestLine = r.Method + " " + r.URL.RequestURI()
		if err := json.NewDecoder(r.Body).Decode(&patch); err != nil {
			t.Fatal(err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{}`)
	})
	if _, err := api.Update(context.Background(), "hh_1", "rec_1", Patch{"amount": "12.00", "account": nil}); err != nil {
		t.Fatal(err)
	}
	if requestLine != "PATCH /api/v1/households/hh_1/recurring/rec_1/" {
		t.Fatalf("request=%q", requestLine)
	}
	if len(patch) != 2 || patch["amount"] != "12.00" {
		t.Fatalf("patch=%#v", patch)
	}
	if account, ok := patch["account"]; !ok || account != nil {
		t.Fatalf("account must be explicit null: %#v", patch)
	}
	if _, ok := patch["category"]; ok {
		t.Fatalf("category must be omitted: %#v", patch)
	}
}

func TestGeneratedRecurringAPIMapsConflictOnExactConfirmURL(t *testing.T) {
	var got string
	api := testRecurringAPI(t, func(w http.ResponseWriter, r *http.Request) {
		got = r.Method + " " + r.URL.RequestURI()
		w.WriteHeader(http.StatusConflict)
	})
	date := daikuv1.RecurringOccurrenceConfirmRequestRequest{}.FinalDate
	_, err := api.Confirm(context.Background(), "hh_1", "occ_1", daikuv1.RecurringOccurrenceConfirmRequestRequest{FinalAmount: "10.00", FinalDate: date})
	cliErr, ok := err.(*cli.Error)
	if !ok || cliErr.ExitCode != cli.ExitConflict || cliErr.Code != "conflict" {
		t.Fatalf("error=%#v", err)
	}
	if got != "POST /api/v1/households/hh_1/recurring/occurrences/occ_1/confirm/" {
		t.Fatalf("request=%q", got)
	}
}

func TestGeneratedRecurringListURL(t *testing.T) {
	var got string
	api := testRecurringAPI(t, func(w http.ResponseWriter, r *http.Request) {
		got = r.Method + " " + r.URL.RequestURI()
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `[]`)
	})
	if _, err := api.List(context.Background(), "hh_1"); err != nil {
		t.Fatal(err)
	}
	if got != "GET /api/v1/households/hh_1/recurring/" {
		t.Fatalf("request=%q", got)
	}
}
