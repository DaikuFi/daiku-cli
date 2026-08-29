package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type sleepFunc func(context.Context, time.Duration) error

type Client struct {
	baseURL         *url.URL
	httpClient      *http.Client
	userAgent       string
	maxResponseBody int64
	maxAttempts     int
	retryDelay      time.Duration
	now             func() time.Time
	sleep           sleepFunc
}

func New(config Config) (*Client, error) {
	base := config.BaseURL
	if base == "" {
		base = DefaultBaseURL
	}
	if !strings.HasSuffix(base, "/") {
		base += "/"
	}
	parsed, err := url.Parse(base)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || !strings.HasSuffix(parsed.Path, "/api/v1/") {
		return nil, fmt.Errorf("invalid API base URL")
	}
	if parsed.Scheme != "https" && parsed.Hostname() != "localhost" && parsed.Hostname() != "127.0.0.1" {
		return nil, fmt.Errorf("API base URL must use HTTPS")
	}
	timeout := config.Timeout
	if timeout == 0 {
		timeout = DefaultTimeout
	}
	httpClient := config.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{}
	}
	// Copy so setting the default timeout never mutates an injected client.
	copyClient := *httpClient
	if copyClient.Timeout == 0 {
		copyClient.Timeout = timeout
	}
	previousRedirectPolicy := copyClient.CheckRedirect
	copyClient.CheckRedirect = safeRedirectPolicy(parsed, previousRedirectPolicy)
	maxBody := config.MaxResponseBody
	if maxBody <= 0 {
		maxBody = DefaultMaxResponseBody
	}
	attempts := config.MaxAttempts
	if attempts <= 0 {
		attempts = DefaultMaxAttempts
	} else if attempts > MaxAttempts {
		attempts = MaxAttempts
	}
	delay := config.RetryDelay
	if delay <= 0 {
		delay = 250 * time.Millisecond
	} else if delay > MaxRetryDelay {
		delay = MaxRetryDelay
	}
	now := config.Now
	if now == nil {
		now = time.Now
	}
	sleep := config.Sleep
	if sleep == nil {
		sleep = sleepContext
	}
	return &Client{parsed, &copyClient, config.UserAgent, maxBody, attempts, delay, now, sleep}, nil
}

// Do sends a request and decodes a successful JSON response into out. A nil
// out is valid for 204 responses. bearerToken is never retained or included in errors.
func (c *Client) Do(ctx context.Context, method, path, bearerToken string, body, out any) error {
	requestURL, err := c.resolve(path)
	if err != nil {
		return newError("invalid_request", "the API request path is invalid", 0, false, nil)
	}
	var payload []byte
	if body != nil {
		payload, err = json.Marshal(body)
		if err != nil {
			return newError("invalid_request", "the API request body could not be encoded", 0, false, err)
		}
	}
	attempts := 1
	if isRetryableMethod(method) {
		attempts = c.maxAttempts
	}
	for attempt := 1; attempt <= attempts; attempt++ {
		err = c.doOnce(ctx, method, requestURL, bearerToken, payload, out)
		if err == nil {
			return err
		}
		var apiErr *Error
		if !errors.As(err, &apiErr) {
			return err
		}
		if !apiErr.Retryable || attempt == attempts {
			return err
		}
		delay := apiErr.RetryAfter
		if delay <= 0 {
			delay = exponentialDelay(c.retryDelay, attempt)
		}
		if err := c.sleep(ctx, delay); err != nil {
			return err
		}
	}
	return err
}

func (c *Client) doOnce(ctx context.Context, method string, requestURL *url.URL, token string, payload []byte, out any) error {
	var body io.Reader
	if payload != nil {
		body = bytes.NewReader(payload)
	}
	req, err := http.NewRequestWithContext(ctx, method, requestURL.String(), body)
	if err != nil {
		return newError("invalid_request", "the API request could not be created", 0, false, err)
	}
	req.Header.Set("Accept", "application/json")
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if c.userAgent != "" {
		req.Header.Set("User-Agent", c.userAgent)
	}
	response, err := c.httpClient.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if errors.Is(err, context.DeadlineExceeded) {
			return newError("timeout", "the Daiku API request timed out", 0, true, err)
		}
		return newError("network_error", "the Daiku API could not be reached", 0, true, err)
	}
	defer response.Body.Close()
	limited := io.LimitReader(response.Body, c.maxResponseBody+1)
	data, readErr := io.ReadAll(limited)
	if readErr != nil {
		return newError("network_error", "the Daiku API response could not be read", response.StatusCode, true, readErr)
	}
	if int64(len(data)) > c.maxResponseBody {
		return newError("response_too_large", "the Daiku API response exceeded the allowed size", response.StatusCode, false, nil)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		apiErr := statusError(response.StatusCode)
		apiErr.RetryAfter = parseRetryAfter(response.Header.Get("Retry-After"), c.now())
		return apiErr
	}
	if response.StatusCode == http.StatusNoContent || out == nil || len(bytes.TrimSpace(data)) == 0 {
		return nil
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if err := decoder.Decode(out); err != nil {
		return newError("invalid_response", "the Daiku API returned an invalid JSON response", response.StatusCode, false, err)
	}
	if err := requireJSONEOF(decoder); err != nil {
		return newError("invalid_response", "the Daiku API returned an invalid JSON response", response.StatusCode, false, err)
	}
	return nil
}

func safeRedirectPolicy(base *url.URL, previous func(*http.Request, []*http.Request) error) func(*http.Request, []*http.Request) error {
	return func(request *http.Request, via []*http.Request) error {
		isSafe := func() bool {
			return request.URL.Scheme == base.Scheme && request.URL.Host == base.Host && strings.HasPrefix(request.URL.Path, base.Path)
		}
		if !isSafe() {
			return http.ErrUseLastResponse
		}
		if previous != nil {
			if err := previous(request, via); err != nil {
				return err
			}
			if !isSafe() {
				return http.ErrUseLastResponse
			}
			return nil
		}
		if len(via) >= 10 {
			return errors.New("stopped after 10 redirects")
		}
		return nil
	}
}

func (c *Client) resolve(path string) (*url.URL, error) {
	path = strings.TrimPrefix(path, "/")
	reference, err := url.Parse(path)
	if err != nil || reference.IsAbs() || reference.Host != "" || strings.HasPrefix(path, "//") || strings.HasPrefix(path, "../") {
		return nil, errors.New("invalid relative path")
	}
	resolved := c.baseURL.ResolveReference(reference)
	if resolved.Scheme != c.baseURL.Scheme || resolved.Host != c.baseURL.Host || !strings.HasPrefix(resolved.Path, c.baseURL.Path) {
		return nil, errors.New("path escapes API base URL")
	}
	return resolved, nil
}

func parseRetryAfter(value string, now time.Time) time.Duration {
	value = strings.TrimSpace(value)
	if seconds, err := strconv.ParseInt(value, 10, 64); err == nil {
		if seconds > 0 {
			if seconds > int64(MaxRetryDelay/time.Second) {
				return MaxRetryDelay
			}
			return time.Duration(seconds) * time.Second
		}
		return 0
	}
	date, err := http.ParseTime(value)
	if err != nil || !date.After(now) {
		return 0
	}
	delay := date.Sub(now)
	if delay > MaxRetryDelay {
		return MaxRetryDelay
	}
	return delay
}

func exponentialDelay(base time.Duration, attempt int) time.Duration {
	delay := base
	for index := 1; index < attempt; index++ {
		if delay >= MaxRetryDelay/2 {
			return MaxRetryDelay
		}
		delay *= 2
	}
	if delay > MaxRetryDelay {
		return MaxRetryDelay
	}
	return delay
}

func requireJSONEOF(decoder *json.Decoder) error {
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return errors.New("multiple JSON values")
		}
		return err
	}
	return nil
}

func isRetryableMethod(method string) bool {
	switch strings.ToUpper(method) {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return true
	default:
		return false
	}
}

func sleepContext(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
