package auth

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/DaikuFi/daiku-cli/internal/credentials"
)

func testClient(t *testing.T, opener func(string) error, handler http.HandlerFunc) *Client {
	t.Helper()
	server := httptest.NewTLSServer(handler)
	t.Cleanup(server.Close)
	client, err := New(Config{ClientID: "cli", AuthorizeURL: server.URL + "/authorize", TokenURL: server.URL + "/token", RevokeURL: server.URL + "/revoke", Scopes: []string{"finance:read", "finance:write"}, HTTPClient: server.Client(), OpenBrowser: opener, Timeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	return client
}
func TestLoginPKCEAndCallback(t *testing.T) {
	var opened string
	var mu sync.Mutex
	client := testClient(t, func(raw string) error {
		mu.Lock()
		opened = raw
		mu.Unlock()
		u, _ := url.Parse(raw)
		callback := u.Query().Get("redirect_uri") + "?code=one-time&state=" + url.QueryEscape(u.Query().Get("state"))
		go http.Get(callback)
		return nil
	}, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/token" {
			http.NotFound(w, r)
			return
		}
		if err := r.ParseForm(); err != nil {
			t.Fatal(err)
		}
		if r.Form.Get("code_verifier") == "" || r.Form.Get("redirect_uri") == "" {
			t.Error("missing PKCE exchange")
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "secret-access", "refresh_token": "secret-refresh", "expires_in": 3600, "scope": "finance:read finance:write"})
	})
	result, err := client.Login(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if result.Token.AccessToken != "secret-access" {
		t.Fatal("wrong token")
	}
	mu.Lock()
	u, _ := url.Parse(opened)
	mu.Unlock()
	if u.Query().Get("code_challenge_method") != "S256" || !strings.Contains(u.Query().Get("redirect_uri"), callbackPath) {
		t.Fatal("invalid authorization URL")
	}
}
func TestLoginRejectsFakeStateWithoutLeakingSecrets(t *testing.T) {
	client := testClient(t, func(raw string) error {
		u, _ := url.Parse(raw)
		go http.Get(u.Query().Get("redirect_uri") + "?code=secret-code&state=fake")
		return nil
	}, func(http.ResponseWriter, *http.Request) { t.Fatal("must not exchange") })
	_, err := client.Login(context.Background())
	if err == nil || strings.Contains(err.Error(), "secret-code") {
		t.Fatalf("unsafe error %v", err)
	}
}
func TestLoginBrowserFailureReturnsManualURLWithoutSecrets(t *testing.T) {
	client := testClient(t, func(string) error { return errors.New("missing") }, func(http.ResponseWriter, *http.Request) {})
	result, err := client.Login(context.Background())
	if err == nil || result.AuthorizationURL == "" {
		t.Fatalf("got %v %#v", err, result)
	}
	if strings.Contains(err.Error(), "code_verifier") {
		t.Fatal("verifier leaked")
	}
}

func TestLoginBrowserFailureCanContinueManually(t *testing.T) {
	client := testClient(t, func(string) error { return errors.New("missing") }, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "access", "refresh_token": "refresh", "expires_in": 3600})
	})
	result, err := client.LoginWithManualFallback(context.Background(), func(raw string) error {
		u, _ := url.Parse(raw)
		go http.Get(u.Query().Get("redirect_uri") + "?code=manual&state=" + url.QueryEscape(u.Query().Get("state")))
		return nil
	})
	if err != nil || result.Token.AccessToken != "access" {
		t.Fatalf("got %#v %v", result, err)
	}
}
func TestLoginDenialAndTimeout(t *testing.T) {
	denied := testClient(t, func(raw string) error {
		u, _ := url.Parse(raw)
		go http.Get(u.Query().Get("redirect_uri") + "?error=access_denied&state=" + url.QueryEscape(u.Query().Get("state")))
		return nil
	}, func(http.ResponseWriter, *http.Request) {})
	if _, err := denied.Login(context.Background()); err == nil {
		t.Fatal("expected denial")
	}
	timeout := testClient(t, func(string) error { return nil }, func(http.ResponseWriter, *http.Request) {})
	timeout.cfg.Timeout = 10 * time.Millisecond
	if _, err := timeout.Login(context.Background()); err == nil {
		t.Fatal("expected timeout")
	}
}

type memoryStore struct {
	mu   sync.Mutex
	t    credentials.Token
	puts int
}

func (m *memoryStore) Get(string) (credentials.Token, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.t, nil
}
func (m *memoryStore) Put(_ string, t credentials.Token) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.t = t
	m.puts++
	return nil
}
func (m *memoryStore) Delete(string) error { return nil }
func TestManagerCoalescesConcurrentRefresh(t *testing.T) {
	var calls int
	var mu sync.Mutex
	client := testClient(t, func(string) error { return nil }, func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		calls++
		mu.Unlock()
		time.Sleep(10 * time.Millisecond)
		_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "new", "refresh_token": "rotated", "expires_in": 3600})
	})
	store := &memoryStore{t: credentials.Token{AccessToken: "old", RefreshToken: "refresh", ExpiresAt: 1}}
	manager := Manager{Store: store, OAuth: client}
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			token, err := manager.AccessToken(context.Background(), "p")
			if err != nil || token != "new" {
				t.Errorf("%q %v", token, err)
			}
		}()
	}
	wg.Wait()
	mu.Lock()
	defer mu.Unlock()
	if calls != 1 {
		t.Fatalf("refresh calls %d", calls)
	}
}
