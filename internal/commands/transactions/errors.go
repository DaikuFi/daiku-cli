package transactions

import "github.com/DaikuFi/daiku-cli/internal/cli"

func safe(code, message string) *cli.Error {
	return &cli.Error{Code: code, Message: message, ExitCode: cli.ExitFailure}
}
func safeStatus(status int, message string) *cli.Error {
	exit := cli.ExitFailure
	switch status {
	case 401:
		exit = cli.ExitAuth
	case 403:
		exit = cli.ExitForbidden
	case 404:
		exit = cli.ExitNotFound
	case 409:
		exit = cli.ExitConflict
	case 429, 500, 502, 503, 504:
		exit = cli.ExitUnavailable
	}
	return &cli.Error{Code: "api_error", Message: message, ExitCode: exit, Details: map[string]any{"status": status}}
}
