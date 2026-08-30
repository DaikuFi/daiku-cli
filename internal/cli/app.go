package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/DaikuFi/daiku-cli/internal/i18n"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"golang.org/x/term"
)

const (
	boldCyan = "\x1b[1;36m"
	reset    = "\x1b[0m"
)

type terminalDetector func(io.Writer) bool
type interactiveDetector func(io.Reader, io.Writer) bool
type terminalWidthDetector func(io.Writer) int

type options struct {
	in            io.Reader
	out           io.Writer
	errOut        io.Writer
	version       string
	modules       []Module
	isTerminal    terminalDetector
	isInteractive interactiveDetector
	terminalWidth terminalWidthDetector
	lookupEnv     func(string) (string, bool)
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

func WithInteractiveDetector(detector func(io.Reader, io.Writer) bool) Option {
	return func(options *options) { options.isInteractive = detector }
}

func WithTerminalWidthDetector(detector func(io.Writer) int) Option {
	return func(options *options) { options.terminalWidth = detector }
}

// WithEnvironment makes locale and NO_COLOR behavior deterministic in tests.
func WithEnvironment(lookup func(string) (string, bool)) Option {
	return func(options *options) { options.lookupEnv = lookup }
}

type App struct {
	options options
}

func New(opts ...Option) *App {
	config := options{
		in:            os.Stdin,
		out:           os.Stdout,
		errOut:        os.Stderr,
		version:       "dev",
		isTerminal:    isTerminal,
		isInteractive: isInteractive,
		terminalWidth: terminalWidth,
		lookupEnv:     os.LookupEnv,
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

func isInteractive(reader io.Reader, writer io.Writer) bool {
	input, inputOK := reader.(*os.File)
	return inputOK && term.IsTerminal(int(input.Fd())) && isTerminal(writer)
}

func terminalWidth(writer io.Writer) int {
	file, ok := writer.(*os.File)
	if !ok {
		return 0
	}
	width, _, err := term.GetSize(int(file.Fd()))
	if err != nil {
		return 80
	}
	return width
}

func (a *App) Run(args []string) int {
	jsonOutput := jsonMode(args)
	language, err := i18n.Resolve(languageMode(args), a.options.lookupEnv)
	if err != nil {
		cliError := usageError(err.Error())
		writeError(a.options.errOut, cliError, jsonOutput, i18n.New(i18n.English))
		return int(cliError.ExitCode)
	}
	localizer := i18n.New(language)
	var helpErr error
	root := a.rootCommand(jsonOutput, localizer, &helpErr)
	_, noColor := a.options.lookupEnv("NO_COLOR")
	root.SetContext(withHumanContext(context.Background(), HumanContext{
		Localizer:   localizer,
		Terminal:    a.options.isTerminal(a.options.out),
		Interactive: a.options.isInteractive(a.options.in, a.options.out),
		Width:       a.options.terminalWidth(a.options.out),
		NoColor:     noColor,
		JSON:        jsonOutput,
	}))
	root.SetArgs(args)
	root.InitDefaultHelpCmd()
	root.InitDefaultCompletionCmd(args...)
	typeCompletionArgsAsUsage(root)

	if _, _, err := root.Find(args); err != nil {
		cliError := usageError(err.Error())
		writeError(a.options.errOut, cliError, jsonOutput, localizer)
		return int(cliError.ExitCode)
	}

	executed, err := root.ExecuteC()
	if err != nil {
		if hasMissingRequiredFlag(executed) {
			err = usageError(err.Error())
		}
		cliError := normalizeError(err)
		writeError(a.options.errOut, cliError, jsonOutput, localizer)
		return int(cliError.ExitCode)
	}
	if helpErr != nil {
		cliError := normalizeError(helpErr)
		writeError(a.options.errOut, cliError, jsonOutput, localizer)
		return int(cliError.ExitCode)
	}

	return int(ExitOK)
}

func (a *App) rootCommand(jsonOutput bool, localizer i18n.Localizer, helpErr *error) *cobra.Command {
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
	root.PersistentFlags().String("language", "", "human output language: en or es")
	root.SetFlagErrorFunc(func(_ *cobra.Command, err error) error {
		return usageError(err.Error())
	})
	root.SetHelpFunc(a.helpFunc(jsonOutput, localizer, helpErr))
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

func (a *App) helpFunc(jsonOutput bool, localizer i18n.Localizer, helpErr *error) func(*cobra.Command, []string) {
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
		_, noColor := a.options.lookupEnv("NO_COLOR")
		if a.options.isTerminal(command.OutOrStdout()) && !noColor {
			heading = boldCyan + heading + reset
		}
		if _, err := fmt.Fprintf(command.OutOrStdout(), "%s\n\n%s\n\n%s:\n  %s\n", heading, localizer.Human(commandDescription(command)), localizer.Text(i18n.UsageHeading), command.UseLine()); err != nil {
			*helpErr = err
			return
		}

		if command.HasAvailableSubCommands() {
			if _, err := fmt.Fprintf(command.OutOrStdout(), "\n%s:\n", localizer.Text(i18n.CommandsHeading)); err != nil {
				*helpErr = err
				return
			}
			for _, child := range command.Commands() {
				if child.IsAvailableCommand() || child.Name() == "help" {
					if _, err := fmt.Fprintf(command.OutOrStdout(), "  %-12s %s\n", child.Name(), localizer.Human(child.Short)); err != nil {
						*helpErr = err
						return
					}
				}
			}
		}

		if _, err := fmt.Fprintf(command.OutOrStdout(), "\n%s:\n", localizer.Text(i18n.FlagsHeading)); err != nil {
			*helpErr = err
			return
		}
		if _, err := fmt.Fprint(command.OutOrStdout(), localizedFlagUsages(command.Flags(), localizer)); err != nil {
			*helpErr = err
		}
	}
}

func localizedFlagUsages(flags *pflag.FlagSet, localizer i18n.Localizer) string {
	original := map[*pflag.Flag]string{}
	flags.VisitAll(func(flag *pflag.Flag) { original[flag] = flag.Usage; flag.Usage = localizer.Human(flag.Usage) })
	usages := flags.FlagUsages()
	for flag, usage := range original {
		flag.Usage = usage
	}
	return usages
}

func languageMode(args []string) string {
	for index, arg := range args {
		if arg == "--" {
			break
		}
		if strings.HasPrefix(arg, "--language=") {
			return strings.TrimPrefix(arg, "--language=")
		}
		if arg == "--language" && index+1 < len(args) {
			return args[index+1]
		}
	}
	return ""
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
