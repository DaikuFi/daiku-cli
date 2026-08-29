package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
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
func TestLoginIgnoresFakeStateThenAcceptsLegitimateCallback(t *testing.T) {
	client := testClient(t, func(raw string) error {
		u, _ := url.Parse(raw)
		go func() {
			_, _ = http.Get(u.Query().Get("redirect_uri") + "?code=secret-code&state=fake")
			_, _ = http.Get(u.Query().Get("redirect_uri") + "?code=legitimate&state=" + url.QueryEscape(u.Query().Get("state")))
		}()
		return nil
	}, func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		if r.Form.Get("code") != "legitimate" {
			t.Fatalf("exchanged fake code")
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "access", "refresh_token": "refresh", "expires_in": 3600})
	})
	result, err := client.Login(context.Background())
	if err != nil || result.Token.AccessToken != "access" || strings.Contains(fmt.Sprint(err), "secret-code") {
		t.Fatalf("unsafe result %#v %v", result, err)
	}
}

func TestCallbackConsumesOnlyOneConcurrentValidTerminalCallback(t *testing.T) {
	results := make(chan callbackResult, 1)
	handler := callbackHandler("expected", 55555, results)
	var wg sync.WaitGroup
	statuses := make(chan int, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(code string) {
			defer wg.Done()
			req := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:55555/oauth/callback?state=expected&code="+code, nil)
			req.Host = "127.0.0.1:55555"
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			statuses <- rec.Code
		}(fmt.Sprintf("code-%d", i))
	}
	wg.Wait()
	close(statuses)
	ok, conflict := 0, 0
	for status := range statuses {
		if status == http.StatusOK {
			ok++
		}
		if status == http.StatusConflict {
			conflict++
		}
	}
	if ok != 1 || conflict != 1 || len(results) != 1 {
		t.Fatalf("ok=%d conflict=%d results=%d", ok, conflict, len(results))
	}
}

func TestOAuthSecretBearingPostsNeverFollowRedirects(t *testing.T) {
	var destinationCalls int
	destination := httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { destinationCalls++ }))
	defer destination.Close()
	source := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, destination.URL, http.StatusTemporaryRedirect)
	}))
	defer source.Close()
	httpClient := source.Client()
	httpClient.Transport = &redirectTransport{source: source.Client().Transport, destination: destination.Client().Transport, destinationHost: destination.Listener.Addr().String()}
	client, err := New(Config{ClientID: "cli", AuthorizeURL: source.URL + "/authorize", TokenURL: source.URL + "/token", RevokeURL: source.URL + "/revoke", HTTPClient: httpClient})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = client.tokenRequest(context.Background(), url.Values{"grant_type": {"authorization_code"}, "code": {"secret-code"}, "code_verifier": {"secret-verifier"}}); err == nil {
		t.Fatal("expected exchange redirect rejection")
	}
	if _, err = client.Refresh(context.Background(), "secret-refresh"); err == nil {
		t.Fatal("expected refresh redirect rejection")
	}
	if err = client.Revoke(context.Background(), "secret-revoke"); err == nil {
		t.Fatal("expected revoke redirect rejection")
	}
	if destinationCalls != 0 {
		t.Fatalf("destination received %d secret-bearing requests", destinationCalls)
	}
}

type redirectTransport struct {
	source, destination http.RoundTripper
	destinationHost     string
}

func (r *redirectTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.URL.Host == r.destinationHost {
		return r.destination.RoundTrip(req)
	}
	return r.source.RoundTrip(req)
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
	manager := Manager{Store: store, OAuth: client, DisableProcessLock: true}
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

func TestManagerReturnsRecoverableCrossProcessRefreshConflict(t *testing.T) {
	manager := Manager{LockDir: filepath.Join(t.TempDir(), "locks")}
	release, err := manager.acquire("work")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = manager.acquire("work"); !errors.Is(err, ErrRefreshInProgress) {
		t.Fatalf("got %v", err)
	}
	release()
	releaseAgain, err := manager.acquire("work")
	if err != nil {
		t.Fatal(err)
	}
	releaseAgain()
}
