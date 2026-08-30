package transactions

import "github.com/DaikuFi/daiku-cli/internal/cli"

func safe(code, message string) *cli.Error {
	return &cli.Error{Code: code, Message: message, ExitCode: cli.ExitFailure}
}
func safeStatus(status int, message string) *cli.Error {
	code := "api_error"
	exit := cli.ExitFailure
	switch status {
	case 400:
		code, exit = "invalid_request", cli.ExitUsage
	case 401:
		code, exit = "unauthorized", cli.ExitAuth
	case 403:
		code, exit = "forbidden", cli.ExitForbidden
	case 404:
		code, exit = "not_found", cli.ExitNotFound
	case 409:
		code, exit = "conflict", cli.ExitConflict
	case 429:
		code, exit = "rate_limited", cli.ExitUnavailable
	case 500, 502, 503, 504:
		code, exit = "api_unavailable", cli.ExitUnavailable
	}
	return &cli.Error{Code: code, Message: message, ExitCode: exit, Details: map[string]any{"status": status}}
}
