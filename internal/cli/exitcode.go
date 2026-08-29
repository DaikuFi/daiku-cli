package cli

// ExitCode is part of Daiku's public automation contract.
type ExitCode int

const (
	ExitOK          ExitCode = 0
	ExitFailure     ExitCode = 1
	ExitUsage       ExitCode = 2
	ExitAuth        ExitCode = 3
	ExitForbidden   ExitCode = 4
	ExitNotFound    ExitCode = 5
	ExitConflict    ExitCode = 6
	ExitUnavailable ExitCode = 7
)

// Error carries a stable machine-readable code and process exit code.
type Error struct {
	Code     string
	Message  string
	ExitCode ExitCode
	Details  any
}

func (e *Error) Error() string { return e.Message }

func usageError(message string) *Error {
	return &Error{Code: "usage_error", Message: message, ExitCode: ExitUsage}
}
