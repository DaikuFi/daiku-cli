package api

import (
	"context"
	"net/http"
	"time"
)

const (
	DefaultBaseURL         = "https://api.daiku.app/api/v1/"
	DefaultTimeout         = 30 * time.Second
	DefaultMaxResponseBody = int64(4 << 20)
	DefaultMaxAttempts     = 3
	// MaxAttempts bounds all automatic attempts, including the initial request.
	MaxAttempts = 5
	// MaxRetryDelay caps configured backoff and server-provided Retry-After values.
	MaxRetryDelay = 30 * time.Second
)

// Config contains transport policy. Authentication is intentionally supplied
// per request so this package does not own or persist credentials.
type Config struct {
	BaseURL         string
	HTTPClient      *http.Client
	Timeout         time.Duration
	UserAgent       string
	MaxResponseBody int64
	MaxAttempts     int
	RetryDelay      time.Duration
	Now             func() time.Time
	Sleep           func(context.Context, time.Duration) error
}
