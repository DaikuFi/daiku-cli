package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"
)

const (
	boldCyan = "\x1b[1;36m"
	reset    = "\x1b[0m"
)

type terminalDetector func(io.Writer) bool

type options struct {
	in         io.Reader
	out        io.Writer
	errOut     io.Writer
	version    string
	modules    []Module
	isTerminal terminalDetector
}

// Option configures an App without relying on package globals.
type Option func(*options)

func WithIO(in io.Reader, out, errOut io.Writer) Option {
	return func(options *options) {
		options.in = in
		options.out = out
		options.errOut = errOut
	}
}

func WithVersion(version string) Option {
	return func(options *options) { options.version = version }
}

func WithModule(module Module) Option {
	return func(options *options) { options.modules = append(options.modules, module) }
}

// WithTerminalDetector is intended for adapters and tests that cannot expose an
// os.File. Normal callers should rely on automatic terminal detection.
func WithTerminalDetector(detector func(io.Writer) bool) Option {
	return func(options *options) { options.isTerminal = detector }
}

type App struct {
	options options
}

func New(opts ...Option) *App {
	config := options{
		in:         os.Stdin,
		out:        os.Stdout,
		errOut:     os.Stderr,
		version:    "dev",
		isTerminal: isTerminal,
	}
	for _, option := range opts {
		option(&config)
	}
	return &App{options: config}
}

func isTerminal(writer io.Writer) bool {
	file, ok := writer.(*os.File)
	return ok && term.IsTerminal(int(file.Fd()))
}

func (a *App) Run(args []string) int {
	jsonOutput := hasJSONFlag(args)
	root := a.rootCommand(jsonOutput)
	root.SetArgs(args)

	if _, err := root.ExecuteC(); err != nil {
		cliError := normalizeError(err)
		writeError(a.options.errOut, cliError, jsonOutput)
		return int(cliError.ExitCode)
	}

	return int(ExitOK)
}

func (a *App) rootCommand(jsonOutput bool) *cobra.Command {
	root := &cobra.Command{
		Use:           "daiku",
		Short:         "Manage Daiku from the command line",
		SilenceErrors: true,
		SilenceUsage:  true,
	}
	root.SetIn(a.options.in)
	root.SetOut(a.options.out)
	root.SetErr(a.options.errOut)
	root.PersistentFlags().Bool("json", false, "write a stable JSON envelope")
	root.SetHelpFunc(a.helpFunc(jsonOutput))

	for _, module := range a.options.modules {
		module.Register(root)
	}

	return root
}

func (a *App) helpFunc(jsonOutput bool) func(*cobra.Command, []string) {
	return func(command *cobra.Command, _ []string) {
		if jsonOutput {
			_ = WriteSuccess(command.OutOrStdout(), map[string]any{
				"command": command.CommandPath(),
				"help":    commandDescription(command),
				"usage":   command.UseLine(),
			})
			return
		}

		heading := "DAIKU"
		if a.options.isTerminal(command.OutOrStdout()) {
			heading = boldCyan + heading + reset
		}
		_, _ = fmt.Fprintf(command.OutOrStdout(), "%s\n\n%s\n\nUsage:\n  %s\n", heading, commandDescription(command), command.UseLine())

		if command.HasAvailableSubCommands() {
			_, _ = fmt.Fprintln(command.OutOrStdout(), "\nCommands:")
			for _, child := range command.Commands() {
				if child.IsAvailableCommand() || child.Name() == "help" {
					_, _ = fmt.Fprintf(command.OutOrStdout(), "  %-12s %s\n", child.Name(), child.Short)
				}
			}
		}

		_, _ = fmt.Fprintln(command.OutOrStdout(), "\nFlags:")
		_, _ = fmt.Fprint(command.OutOrStdout(), command.Flags().FlagUsages())
	}
}

func commandDescription(command *cobra.Command) string {
	if command.Long != "" {
		return command.Long
	}
	return command.Short
}

func hasJSONFlag(args []string) bool {
	for _, arg := range args {
		if arg == "--json" || arg == "--json=true" {
			return true
		}
	}
	return false
}

func normalizeError(err error) *Error {
	var cliError *Error
	if errors.As(err, &cliError) {
		return cliError
	}

	message := err.Error()
	if strings.HasPrefix(message, "unknown command") || strings.HasPrefix(message, "unknown flag") || strings.Contains(message, "requires") {
		return usageError(message)
	}

	return &Error{Code: "internal_error", Message: message, ExitCode: ExitFailure}
}
