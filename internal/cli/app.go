package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
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
	jsonOutput := jsonMode(args)
	var helpErr error
	root := a.rootCommand(jsonOutput, &helpErr)
	root.SetArgs(args)
	root.InitDefaultHelpCmd()
	root.InitDefaultCompletionCmd(args...)
	typeCompletionArgsAsUsage(root)

	if _, _, err := root.Find(args); err != nil {
		cliError := usageError(err.Error())
		writeError(a.options.errOut, cliError, jsonOutput)
		return int(cliError.ExitCode)
	}

	executed, err := root.ExecuteC()
	if err != nil {
		if hasMissingRequiredFlag(executed) {
			err = usageError(err.Error())
		}
		cliError := normalizeError(err)
		writeError(a.options.errOut, cliError, jsonOutput)
		return int(cliError.ExitCode)
	}
	if helpErr != nil {
		cliError := normalizeError(helpErr)
		writeError(a.options.errOut, cliError, jsonOutput)
		return int(cliError.ExitCode)
	}

	return int(ExitOK)
}

func (a *App) rootCommand(jsonOutput bool, helpErr *error) *cobra.Command {
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
	root.SetFlagErrorFunc(func(_ *cobra.Command, err error) error {
		return usageError(err.Error())
	})
	root.SetHelpFunc(a.helpFunc(jsonOutput, helpErr))
	root.SetHelpCommand(newHelpCommand(root))

	for _, module := range a.options.modules {
		module.Register(root)
	}

	return root
}

func newHelpCommand(root *cobra.Command) *cobra.Command {
	return &cobra.Command{
		Use:   "help [command]",
		Short: "Help about any command",
		Long:  "Help provides help for any command in the application.",
		RunE: func(_ *cobra.Command, args []string) error {
			command, _, err := root.Find(args)
			if err != nil || command == nil {
				if err == nil {
					err = fmt.Errorf("unknown help topic %q", args)
				}
				return usageError(err.Error())
			}
			command.InitDefaultHelpFlag()
			command.InitDefaultVersionFlag()
			return command.Help()
		},
	}
}

func typeCompletionArgsAsUsage(root *cobra.Command) {
	completion, _, err := root.Find([]string{"completion"})
	if err != nil || completion == nil || completion.Name() != "completion" {
		return
	}
	var wrap func(*cobra.Command)
	wrap = func(command *cobra.Command) {
		if command.Args != nil {
			command.Args = UsageArgs(command.Args)
		}
		for _, child := range command.Commands() {
			wrap(child)
		}
	}
	wrap(completion)
}

func (a *App) helpFunc(jsonOutput bool, helpErr *error) func(*cobra.Command, []string) {
	return func(command *cobra.Command, _ []string) {
		if *helpErr != nil {
			return
		}
		if jsonOutput {
			*helpErr = WriteSuccess(command.OutOrStdout(), map[string]any{
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
		if _, err := fmt.Fprintf(command.OutOrStdout(), "%s\n\n%s\n\nUsage:\n  %s\n", heading, commandDescription(command), command.UseLine()); err != nil {
			*helpErr = err
			return
		}

		if command.HasAvailableSubCommands() {
			if _, err := fmt.Fprintln(command.OutOrStdout(), "\nCommands:"); err != nil {
				*helpErr = err
				return
			}
			for _, child := range command.Commands() {
				if child.IsAvailableCommand() || child.Name() == "help" {
					if _, err := fmt.Fprintf(command.OutOrStdout(), "  %-12s %s\n", child.Name(), child.Short); err != nil {
						*helpErr = err
						return
					}
				}
			}
		}

		if _, err := fmt.Fprintln(command.OutOrStdout(), "\nFlags:"); err != nil {
			*helpErr = err
			return
		}
		if _, err := fmt.Fprint(command.OutOrStdout(), command.Flags().FlagUsages()); err != nil {
			*helpErr = err
		}
	}
}

func commandDescription(command *cobra.Command) string {
	if command.Long != "" {
		return command.Long
	}
	return command.Short
}

// jsonMode mirrors pflag's bool syntax. It is intentionally small: Cobra may
// fail before a command or persistent flag value can be inspected, but error
// rendering still needs to know whether the caller requested JSON.
func jsonMode(args []string) bool {
	enabled := false
	for _, arg := range args {
		if arg == "--" {
			break
		}
		if arg == "--json" {
			enabled = true
			continue
		}
		if strings.HasPrefix(arg, "--json=") {
			value, err := strconv.ParseBool(strings.TrimPrefix(arg, "--json="))
			if err != nil {
				return false
			}
			enabled = value
		}
	}
	return enabled
}

func normalizeError(err error) *Error {
	var cliError *Error
	if errors.As(err, &cliError) {
		return cliError
	}

	return &Error{Code: "internal_error", Message: "an unexpected internal error occurred", ExitCode: ExitFailure}
}

func hasMissingRequiredFlag(command *cobra.Command) bool {
	if command == nil {
		return false
	}
	missing := false
	command.Flags().VisitAll(func(flag *pflag.Flag) {
		if flag.Changed {
			return
		}
		for _, annotation := range flag.Annotations[cobra.BashCompOneRequiredFlag] {
			if annotation == "true" {
				missing = true
			}
		}
	})
	return missing
}
