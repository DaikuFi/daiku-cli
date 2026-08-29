package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os/exec"
	"runtime"
	"strings"
	"sync/atomic"
	"time"

	"github.com/DaikuFi/daiku-cli/internal/credentials"
)

const callbackPath = "/oauth/callback"

type Config struct {
	ClientID, AuthorizeURL, TokenURL, RevokeURL string
	Scopes                                      []string
	HTTPClient                                  *http.Client
	OpenBrowser                                 func(string) error
	Timeout                                     time.Duration
}

type Client struct{ cfg Config }

func New(cfg Config) (*Client, error) {
	for _, raw := range []string{cfg.AuthorizeURL, cfg.TokenURL, cfg.RevokeURL} {
		u, err := url.Parse(raw)
		if err != nil || u.Scheme != "https" || u.Host == "" {
			return nil, errors.New("OAuth endpoints must be absolute HTTPS URLs")
		}
	}
	if cfg.ClientID == "" {
		return nil, errors.New("OAuth client ID is required")
	}
	if cfg.HTTPClient == nil {
		cfg.HTTPClient = &http.Client{Timeout: 30 * time.Second}
	}
	if cfg.OpenBrowser == nil {
		cfg.OpenBrowser = openBrowser
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 3 * time.Minute
	}
	return &Client{cfg}, nil
}

type LoginResult struct {
	Token            credentials.Token
	AuthorizationURL string
}

func (c *Client) Login(ctx context.Context) (LoginResult, error) {
	return c.LoginWithManualFallback(ctx, nil)
}

// LoginWithManualFallback keeps the callback listener alive when launching a
// browser fails and gives an interactive caller the URL to open itself.
func (c *Client) LoginWithManualFallback(ctx context.Context, manual func(string) error) (LoginResult, error) {
	listener, port, err := listenLoopback()
	if err != nil {
		return LoginResult{}, errors.New("no OAuth callback port is available")
	}
	defer listener.Close()
	state, err := randomURLSafe(32)
	if err != nil {
		return LoginResult{}, errors.New("could not create OAuth state")
	}
	verifier, err := randomURLSafe(64)
	if err != nil {
		return LoginResult{}, errors.New("could not create PKCE verifier")
	}
	redirect := fmt.Sprintf("http://127.0.0.1:%d%s", port, callbackPath)
	authURL, _ := url.Parse(c.cfg.AuthorizeURL)
	query := authURL.Query()
	query.Set("response_type", "code")
	query.Set("client_id", c.cfg.ClientID)
	query.Set("redirect_uri", redirect)
	query.Set("scope", strings.Join(c.cfg.Scopes, " "))
	query.Set("state", state)
	query.Set("code_challenge_method", "S256")
	sum := sha256.Sum256([]byte(verifier))
	query.Set("code_challenge", base64.RawURLEncoding.EncodeToString(sum[:]))
	authURL.RawQuery = query.Encode()
	result := make(chan callbackResult, 1)
	server := &http.Server{ReadHeaderTimeout: 5 * time.Second, Handler: callbackHandler(state, port, result)}
	go func() { _ = server.Serve(listener) }()
	defer server.Shutdown(context.Background())
	if err := c.cfg.OpenBrowser(authURL.String()); err != nil {
		if manual == nil {
			return LoginResult{AuthorizationURL: authURL.String()}, fmt.Errorf("browser unavailable; open the authorization URL manually: %w", err)
		}
		if notifyErr := manual(authURL.String()); notifyErr != nil {
			return LoginResult{}, errors.New("authorization URL could not be displayed")
		}
	}
	waitCtx, cancel := context.WithTimeout(ctx, c.cfg.Timeout)
	defer cancel()
	select {
	case <-waitCtx.Done():
		return LoginResult{}, errors.New("OAuth login timed out or was canceled")
	case cb := <-result:
		if cb.err != nil {
			return LoginResult{}, cb.err
		}
		token, err := c.exchange(waitCtx, cb.code, verifier, redirect)
		return LoginResult{Token: token}, err
	}
}

type callbackResult struct {
	code string
	err  error
}

func callbackHandler(state string, port int, result chan<- callbackResult) http.Handler {
	var used atomic.Bool
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != callbackPath || r.Host != fmt.Sprintf("127.0.0.1:%d", port) || !used.CompareAndSwap(false, true) {
			http.Error(w, "Invalid callback", http.StatusBadRequest)
			return
		}
		if values := r.URL.Query()["state"]; len(values) != 1 || values[0] != state {
			result <- callbackResult{err: errors.New("OAuth callback state did not match")}
			http.Error(w, "Invalid state", http.StatusBadRequest)
			return
		}
		if denied := r.URL.Query().Get("error"); denied != "" {
			result <- callbackResult{err: errors.New("OAuth authorization was denied")}
			http.Error(w, "Authorization denied", http.StatusBadRequest)
			return
		}
		code := r.URL.Query().Get("code")
		if code == "" {
			result <- callbackResult{err: errors.New("OAuth callback did not include a code")}
			http.Error(w, "Missing code", http.StatusBadRequest)
			return
		}
		result <- callbackResult{code: code}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = io.WriteString(w, "Daiku CLI authorization complete. You may close this window.\n")
	})
}

func listenLoopback() (net.Listener, int, error) {
	for i := 0; i < 32; i++ {
		raw := make([]byte, 2)
		if _, err := rand.Read(raw); err != nil {
			return nil, 0, err
		}
		port := 49152 + (int(raw[0])<<8|int(raw[1]))%(65535-49152+1)
		ln, err := net.Listen("tcp4", fmt.Sprintf("127.0.0.1:%d", port))
		if err == nil {
			return ln, port, nil
		}
	}
	return nil, 0, errors.New("ports occupied")
}
func randomURLSafe(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func (c *Client) exchange(ctx context.Context, code, verifier, redirect string) (credentials.Token, error) {
	return c.tokenRequest(ctx, url.Values{"grant_type": {"authorization_code"}, "client_id": {c.cfg.ClientID}, "code": {code}, "code_verifier": {verifier}, "redirect_uri": {redirect}})
}
func (c *Client) Refresh(ctx context.Context, refresh string) (credentials.Token, error) {
	if refresh == "" {
		return credentials.Token{}, errors.New("refresh token is missing")
	}
	return c.tokenRequest(ctx, url.Values{"grant_type": {"refresh_token"}, "client_id": {c.cfg.ClientID}, "refresh_token": {refresh}})
}
func (c *Client) tokenRequest(ctx context.Context, form url.Values) (credentials.Token, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.cfg.TokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return credentials.Token{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := c.cfg.HTTPClient.Do(req)
	if err != nil {
		return credentials.Token{}, errors.New("OAuth service is unavailable")
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return credentials.Token{}, errors.New("OAuth response could not be read")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return credentials.Token{}, errors.New("OAuth credentials were rejected")
	}
	var wire struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int64  `json:"expires_in"`
		Scope        string `json:"scope"`
	}
	if json.Unmarshal(body, &wire) != nil || wire.AccessToken == "" {
		return credentials.Token{}, errors.New("OAuth service returned an invalid response")
	}
	return credentials.Token{AccessToken: wire.AccessToken, RefreshToken: wire.RefreshToken, ExpiresAt: time.Now().Add(time.Duration(wire.ExpiresIn) * time.Second).Unix(), Scope: wire.Scope}, nil
}
func (c *Client) Revoke(ctx context.Context, token string) error {
	if token == "" {
		return nil
	}
	form := url.Values{"client_id": {c.cfg.ClientID}, "token": {token}, "token_type_hint": {"refresh_token"}}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.cfg.RevokeURL, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := c.cfg.HTTPClient.Do(req)
	if err != nil {
		return errors.New("OAuth service is unavailable")
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return errors.New("OAuth token could not be revoked")
	}
	return nil
}
func openBrowser(target string) error {
	var command string
	var args []string
	if runtime.GOOS == "darwin" {
		command = "open"
		args = []string{target}
	} else {
		command = "xdg-open"
		args = []string{target}
	}
	return exec.Command(command, args...).Start()
}
