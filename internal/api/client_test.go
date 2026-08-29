package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestDoSuccessAndRequestContract(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/accounts/" {
			t.Errorf("path = %q", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer secret" {
			t.Errorf("authorization = %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"amount":1234567890.123456789}`)
	}))
	defer server.Close()
	client := testClient(t, server.URL+"/api/v1/", Config{})
	var response map[string]any
	if err := client.Do(context.Background(), http.MethodGet, "/accounts/", "secret", nil, &response); err != nil {
		t.Fatal(err)
	}
	amount, ok := response["amount"].(json.Number)
	if !ok || amount.String() != "1234567890.123456789" {
		t.Fatalf("decimal was coerced: %#v", response["amount"])
	}
}

func TestDoNoContent(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }))
	defer server.Close()
	client := testClient(t, server.URL+"/api/v1/", Config{})
	if err := client.Do(context.Background(), http.MethodDelete, "resource/1/", "", nil, nil); err != nil {
		t.Fatal(err)
	}
}

func TestDoRejectsPathsOutsideAPIBase(t *testing.T) {
	t.Parallel()
	client := testClient(t, "https://example.test/api/v1/", Config{})
	for _, path := range []string{"../admin/", "accounts/../../../admin/", "https://evil.test/api/v1/"} {
		err := client.Do(context.Background(), http.MethodGet, path, "secret", nil, nil)
		assertAPIError(t, err, "invalid_request")
	}
}

func TestDoInvalidJSONAndBoundedBody(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name, body, code string
		limit            int64
	}{{"invalid", "not-json", "invalid_response", 100}, {"multiple values", `{} {}`, "invalid_response", 100}, {"trailing garbage", `{} x`, "invalid_response", 100}, {"large", "123456", "response_too_large", 5}} {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, test.body) }))
			defer server.Close()
			client := testClient(t, server.URL+"/api/v1/", Config{MaxResponseBody: test.limit})
			var out any
			err := client.Do(context.Background(), http.MethodGet, "x/", "", nil, &out)
			assertAPIError(t, err, test.code)
		})
	}
}

func TestRedirectsCannotEscapeCredentialBoundary(t *testing.T) {
	t.Parallel()
	var escapedRequests atomic.Int32
	escape := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		escapedRequests.Add(1)
		if r.Header.Get("Authorization") != "" {
			t.Error("authorization reached redirect target")
		}
	}))
	defer escape.Close()

	for _, test := range []struct {
		name, location string
	}{{"cross origin", escape.URL + "/api/v1/stolen/"}, {"same origin path escape", "/admin/"}} {
		t.Run(test.name, func(t *testing.T) {
			origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				http.Redirect(w, r, test.location, http.StatusFound)
			}))
			defer origin.Close()
			client := testClient(t, origin.URL+"/api/v1/", Config{})
			err := client.Do(context.Background(), http.MethodGet, "accounts/", "secret", nil, nil)
			assertAPIError(t, err, "api_error")
		})
	}
	if escapedRequests.Load() != 0 {
		t.Fatalf("redirect target received %d requests", escapedRequests.Load())
	}
}

func TestRedirectPolicyRejectsSubdomainAndAllowsAPIPath(t *testing.T) {
	t.Parallel()
	base, err := url.Parse("https://api.daiku.app/api/v1/")
	if err != nil {
		t.Fatal(err)
	}
	policy := safeRedirectPolicy(base, nil)
	for _, target := range []string{"https://sub.api.daiku.app/api/v1/x/", "https://api.daiku.app/admin/"} {
		parsed, parseErr := url.Parse(target)
		if parseErr != nil {
			t.Fatal(parseErr)
		}
		if err := policy(&http.Request{URL: parsed}, nil); !errors.Is(err, http.ErrUseLastResponse) {
			t.Fatalf("target %q: error = %v", target, err)
		}
	}
	allowed, _ := url.Parse("https://api.daiku.app/api/v1/accounts/")
	if err := policy(&http.Request{URL: allowed}, nil); err != nil {
		t.Fatalf("safe redirect rejected: %v", err)
	}
}

func TestDoMapsHTTPFailuresWithoutLeakingResponseOrToken(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		status int
		code   string
	}{{400, "api_error"}, {401, "authentication_required"}, {403, "forbidden"}, {404, "not_found"}, {409, "conflict"}} {
		t.Run(test.code, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(test.status)
				_, _ = io.WriteString(w, `{"detail":"private server detail"}`)
			}))
			defer server.Close()
			client := testClient(t, server.URL+"/api/v1/", Config{})
			err := client.Do(context.Background(), http.MethodPost, "x/", "sensitive-token", map[string]string{"secret": "value"}, nil)
			apiErr := assertAPIError(t, err, test.code)
			if strings.Contains(apiErr.Error(), "private") || strings.Contains(apiErr.Error(), "sensitive") {
				t.Fatalf("error leaked sensitive data: %v", apiErr)
			}
		})
	}
}

func TestRetryAfterDeltaAndDateForSafeRequest(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 29, 12, 0, 0, 0, time.UTC)
	for _, test := range []struct {
		name, header string
		want         time.Duration
	}{{"seconds", "7", 7 * time.Second}, {"date", now.Add(11 * time.Second).Format(http.TimeFormat), 11 * time.Second}} {
		t.Run(test.name, func(t *testing.T) {
			var attempts atomic.Int32
			var delays []time.Duration
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				if attempts.Add(1) == 1 {
					w.Header().Set("Retry-After", test.header)
					w.WriteHeader(http.StatusTooManyRequests)
					return
				}
				_, _ = io.WriteString(w, `{}`)
			}))
			defer server.Close()
			client := testClient(t, server.URL+"/api/v1/", Config{MaxAttempts: 2, Now: func() time.Time { return now }, Sleep: func(_ context.Context, delay time.Duration) error { delays = append(delays, delay); return nil }})
			var out map[string]any
			if err := client.Do(context.Background(), http.MethodGet, "x/", "", nil, &out); err != nil {
				t.Fatal(err)
			}
			if attempts.Load() != 2 || len(delays) != 1 || delays[0] != test.want {
				t.Fatalf("attempts=%d delays=%v", attempts.Load(), delays)
			}
		})
	}
}

func TestMutationIsNeverRetried(t *testing.T) {
	t.Parallel()
	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		attempts.Add(1)
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()
	client := testClient(t, server.URL+"/api/v1/", Config{MaxAttempts: 3})
	err := client.Do(context.Background(), http.MethodPost, "x/", "", map[string]string{"name": "x"}, nil)
	assertAPIError(t, err, "api_unavailable")
	if attempts.Load() != 1 {
		t.Fatalf("unsafe mutation attempted %d times", attempts.Load())
	}
}

func TestSafeRequestRetries5xxWithDeterministicBackoff(t *testing.T) {
	t.Parallel()
	var attempts atomic.Int32
	var delays []time.Duration
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { attempts.Add(1); w.WriteHeader(http.StatusBadGateway) }))
	defer server.Close()
	client := testClient(t, server.URL+"/api/v1/", Config{MaxAttempts: 3, RetryDelay: time.Second, Sleep: func(_ context.Context, delay time.Duration) error { delays = append(delays, delay); return nil }})
	err := client.Do(context.Background(), http.MethodGet, "x/", "", nil, nil)
	apiErr := assertAPIError(t, err, "api_unavailable")
	if !apiErr.Retryable || attempts.Load() != 3 || len(delays) != 2 || delays[0] != time.Second || delays[1] != 2*time.Second {
		t.Fatalf("attempts=%d delays=%v error=%+v", attempts.Load(), delays, apiErr)
	}
}

func TestRetryPolicyIsBounded(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 29, 12, 0, 0, 0, time.UTC)
	for _, test := range []struct {
		name, value string
		want        time.Duration
	}{{"negative", "-1", 0}, {"invalid", "soon", 0}, {"max int", "9223372036854775807", MaxRetryDelay}, {"overflow", "999999999999999999999999", 0}, {"far future", now.Add(24 * time.Hour).Format(http.TimeFormat), MaxRetryDelay}} {
		t.Run(test.name, func(t *testing.T) {
			if got := parseRetryAfter(test.value, now); got != test.want {
				t.Fatalf("got %v, want %v", got, test.want)
			}
		})
	}
	var attempts atomic.Int32
	var delays []time.Duration
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		attempts.Add(1)
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()
	client := testClient(t, server.URL+"/api/v1/", Config{MaxAttempts: 1 << 30, RetryDelay: 24 * time.Hour, Sleep: func(_ context.Context, delay time.Duration) error { delays = append(delays, delay); return nil }})
	assertAPIError(t, client.Do(context.Background(), http.MethodGet, "x/", "", nil, nil), "api_unavailable")
	if attempts.Load() != MaxAttempts {
		t.Fatalf("attempts = %d", attempts.Load())
	}
	for _, delay := range delays {
		if delay != MaxRetryDelay {
			t.Fatalf("delay = %v", delay)
		}
	}
}

func TestNewRejectsUnsafeBaseURLComponents(t *testing.T) {
	t.Parallel()
	for _, base := range []string{"https://user:pass@example.test/api/v1/", "https://example.test/api/v1/?token=secret", "https://example.test/api/v1/#fragment", "https://example.test/"} {
		if _, err := New(Config{BaseURL: base}); err == nil {
			t.Fatalf("accepted %q", base)
		}
	}
}

func TestTimeoutNetworkAndCancellation(t *testing.T) {
	t.Parallel()
	t.Run("timeout", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { time.Sleep(50 * time.Millisecond) }))
		defer server.Close()
		client := testClient(t, server.URL+"/api/v1/", Config{Timeout: 5 * time.Millisecond, MaxAttempts: 1})
		if client.httpClient.Timeout != 5*time.Millisecond {
			t.Fatalf("timeout = %v", client.httpClient.Timeout)
		}
		err := client.Do(context.Background(), http.MethodGet, "x/", "", nil, nil)
		assertAPIError(t, err, "timeout")
	})
	t.Run("network", func(t *testing.T) {
		client := testClient(t, "http://localhost/api/v1/", Config{HTTPClient: &http.Client{Transport: failingTransport{}}, MaxAttempts: 1})
		assertAPIError(t, client.Do(context.Background(), http.MethodGet, "x/", "", nil, nil), "network_error")
	})
	t.Run("canceled", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		client := testClient(t, "http://localhost/api/v1/", Config{HTTPClient: &http.Client{Transport: failingTransport{}}, MaxAttempts: 3})
		if err := client.Do(ctx, http.MethodGet, "x/", "", nil, nil); !errors.Is(err, context.Canceled) {
			t.Fatalf("error = %v", err)
		}
	})
}

type failingTransport struct{}

func (failingTransport) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, errors.New("dial failed with sensitive.internal")
}

func testClient(t *testing.T, base string, config Config) *Client {
	t.Helper()
	config.BaseURL = base
	client, err := New(config)
	if err != nil {
		t.Fatal(err)
	}
	return client
}

func assertAPIError(t *testing.T, err error, code string) *Error {
	t.Helper()
	var apiErr *Error
	if !errors.As(err, &apiErr) {
		t.Fatalf("error = %T %v", err, err)
	}
	if apiErr.Code != code {
		t.Fatalf("code = %q, want %q", apiErr.Code, code)
	}
	return apiErr
}
