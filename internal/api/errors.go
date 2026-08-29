package api

import (
	"fmt"
	"time"
)

// Error is a stable, caller-safe API failure. Body snippets, URLs, headers and
// credentials are deliberately excluded from Error() and its exported fields.
type Error struct {
	Code       string
	Message    string
	StatusCode int
	Retryable  bool
	RetryAfter time.Duration
	cause      error
}

func (e *Error) Error() string { return e.Message }
func (e *Error) Unwrap() error { return e.cause }

func newError(code, message string, status int, retryable bool, cause error) *Error {
	return &Error{Code: code, Message: message, StatusCode: status, Retryable: retryable, cause: cause}
}

func statusError(status int) *Error {
	switch status {
	case 401:
		return newError("authentication_required", "authentication is required or expired", status, false, nil)
	case 403:
		return newError("forbidden", "you do not have permission to perform this operation", status, false, nil)
	case 404:
		return newError("not_found", "the requested resource was not found", status, false, nil)
	case 409:
		return newError("conflict", "the request conflicts with the current state", status, false, nil)
	case 429:
		return newError("rate_limited", "the API rate limit was exceeded", status, true, nil)
	default:
		if status >= 500 {
			return newError("api_unavailable", "the Daiku API is temporarily unavailable", status, true, nil)
		}
		return newError("api_error", fmt.Sprintf("the Daiku API rejected the request (status %d)", status), status, false, nil)
	}
}
