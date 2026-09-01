package cli_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/DaikuFi/daiku-cli/internal/agent"
	"github.com/DaikuFi/daiku-cli/internal/cli"
	versioncommand "github.com/DaikuFi/daiku-cli/internal/commands/version"
	"github.com/spf13/cobra"
)

type defaultableFlagModule struct{}

func (defaultableFlagModule) Register(root *cobra.Command) {
	command := &cobra.Command{
		Use: "scoped",
		RunE: func(cmd *cobra.Command, _ []string) error {
			value, err := cmd.Flags().GetString("household")
			if err != nil {
				return err
			}
			return cli.WriteSuccess(cmd.OutOrStdout(), map[string]string{"household": value})
		},
	}
	command.Flags().String("household", "", "household ID")
	_ = command.MarkFlagRequired("household")
	root.AddCommand(command)
}

func TestFlagDefaultFillsRequiredFlagAndExplicitValueWins(t *testing.T) {
	for _, test := range []struct {
		name string
		args []string
		want string
	}{
		{name: "selected", args: []string{"scoped", "--json"}, want: "hsh_selected"},
		{name: "explicit", args: []string{"scoped", "--household", "hsh_explicit", "--json"}, want: "hsh_explicit"},
	} {
		t.Run(test.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			calls := 0
			app := cli.New(
				cli.WithIO(strings.NewReader(""), &stdout, &stderr),
				cli.WithFlagDefault("household", func(context.Context) (string, error) {
					calls++
					return "hsh_selected", nil
				}),
				cli.WithModule(defaultableFlagModule{}),
			)
			if exit := app.Run(test.args); exit != int(cli.ExitOK) || stderr.Len() != 0 || !strings.Contains(stdout.String(), `"household":"`+test.want+`"`) {
				t.Fatalf("exit=%d stdout=%q stderr=%q", exit, stdout.String(), stderr.String())
			}
			if test.name == "explicit" && calls != 0 {
				t.Fatalf("resolver called %d times", calls)
			}
		})
	}
}

func TestFlagDefaultIsOptionalInAgentMetadata(t *testing.T) {
	app := cli.New(
		cli.WithFlagDefault("household", func(context.Context) (string, error) { return "hsh_selected", nil }),
		cli.WithModule(defaultableFlagModule{}),
	)
	for _, command := range app.Commands() {
		if command.Path != "daiku scoped" {
			continue
		}
		for _, flag := range command.Flags {
			if flag.Name == "household" && flag.Required {
				t.Fatalf("defaultable flag is still required: %+v", flag)
			}
			if flag.Name == "household" {
				return
			}
		}
	}
	t.Fatal("household flag not found")
}

func TestFlagDefaultRejectsExplicitEmptyValue(t *testing.T) {
	var stdout, stderr bytes.Buffer
	app := cli.New(
		cli.WithIO(strings.NewReader(""), &stdout, &stderr),
		cli.WithFlagDefault("household", func(context.Context) (string, error) { return "hsh_selected", nil }),
		cli.WithModule(defaultableFlagModule{}),
	)
	if exit := app.Run([]string{"scoped", "--household=", "--json"}); exit != int(cli.ExitUsage) || stdout.Len() != 0 || !strings.Contains(stderr.String(), `"code":"usage_error"`) {
		t.Fatalf("exit=%d stdout=%q stderr=%q", exit, stdout.String(), stderr.String())
	}
}

func run(t *testing.T, terminal bool, args ...string) (int, string, string) {
	t.Helper()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	app := cli.New(
		cli.WithIO(strings.NewReader(""), &stdout, &stderr),
		cli.WithVersion("1.2.3"),
		cli.WithModule(versioncommand.New("1.2.3")),
		cli.WithModule(failureModule{}),
		cli.WithModule(internalFailureModule{}),
		cli.WithModule(requiredFlagModule{}),
		cli.WithTerminalDetector(func(_ io.Writer) bool { return terminal }),
		cli.WithEnvironment(func(string) (string, bool) { return "", false }),
	)

	return app.Run(args), stdout.String(), stderr.String()
}

type failureModule struct{}

func (failureModule) Register(root *cobra.Command) {
	root.AddCommand(&cobra.Command{
		Use:    "fail",
		Short:  "Return a typed failure in tests",
		Hidden: true,
		RunE: func(_ *cobra.Command, _ []string) error {
			return &cli.Error{
				Code:     "authentication_required",
				Message:  "sign in first",
				ExitCode: cli.ExitAuth,
				Details:  map[string]string{"action": "daiku auth login"},
			}
		},
	})
}

type internalFailureModule struct{}

func (internalFailureModule) Register(root *cobra.Command) {
	root.AddCommand(&cobra.Command{
		Use:    "internal-fail",
		Short:  "Return an untyped failure in tests",
		Hidden: true,
		RunE: func(_ *cobra.Command, _ []string) error {
			return errors.New("invalid argument: database password leaked in underlying error")
		},
	})
}

type requiredFlagModule struct{}

func (requiredFlagModule) Register(root *cobra.Command) {
	command := &cobra.Command{
		Use:    "required",
		Short:  "Require a flag in tests",
		Hidden: true,
		RunE: func(_ *cobra.Command, _ []string) error {
			return nil
		},
	}
	command.Flags().String("token", "", "required test token")
	_ = command.MarkFlagRequired("token")
	root.AddCommand(command)
}

func TestRootHelpForPipeIsDeterministicAndPlain(t *testing.T) {
	exitCode, stdout, stderr := run(t, false, "--help")

	if exitCode != int(cli.ExitOK) {
		t.Fatalf("exit code = %d, want %d", exitCode, cli.ExitOK)
	}
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty", stderr)
	}
	if strings.Contains(stdout, "\x1b[") {
		t.Fatalf("pipe output contains ANSI escapes: %q", stdout)
	}
	for _, want := range []string{"DAIKU", "Usage:", "version", "--json"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("help output missing %q: %s", want, stdout)
		}
	}
}

func TestRootHelpForTerminalUsesHumanStyling(t *testing.T) {
	exitCode, stdout, _ := run(t, true, "--help")

	if exitCode != int(cli.ExitOK) {
		t.Fatalf("exit code = %d, want %d", exitCode, cli.ExitOK)
	}
	if !strings.Contains(stdout, "\x1b[1;36mDAIKU\x1b[0m") {
		t.Fatalf("terminal help is not styled: %q", stdout)
	}
}

func TestRootHelpSpanishGolden(t *testing.T) {
	exitCode, stdout, stderr := run(t, false, "--language", "es", "--help")
	want := "DAIKU\n\nGestiona Daiku desde la línea de comandos\n\nUso:\n  daiku [flags]\n\nComandos:\n" +
		"  commands     Lista metadatos de comandos para máquinas\n" +
		"  completion   Genera el script de autocompletado para el shell indicado\n" +
		"  help         Ayuda sobre cualquier comando\n" +
		"  version      Muestra la versión del CLI de Daiku\n\nOpciones:\n" +
		"      --agent             habilita salida JSON segura para agentes y desactiva la entrada\n" +
		"  -h, --help              ayuda de daiku\n" +
		"      --json              escribe un envelope JSON estable\n" +
		"      --language string   idioma de salida humana: en o es\n" +
		"      --no-input          desactiva toda entrada interactiva\n" +
		"  -v, --version           versión de daiku\n"
	if exitCode != int(cli.ExitOK) || stderr != "" || stdout != want {
		t.Fatalf("exit=%d stderr=%q\noutput:\n%q\nwant:\n%q", exitCode, stderr, stdout, want)
	}
}

func TestLocaleEnvironmentAndNoColor(t *testing.T) {
	var stdout, stderr bytes.Buffer
	environment := map[string]string{"LANG": "es_UY.UTF-8", "NO_COLOR": "1"}
	app := cli.New(
		cli.WithIO(strings.NewReader(""), &stdout, &stderr),
		cli.WithTerminalDetector(func(io.Writer) bool { return true }),
		cli.WithEnvironment(func(key string) (string, bool) { value, ok := environment[key]; return value, ok }),
	)
	exitCode := app.Run([]string{"--help"})
	if exitCode != int(cli.ExitOK) || !strings.Contains(stdout.String(), "Uso:") || strings.Contains(stdout.String(), "\x1b[") {
		t.Fatalf("exit=%d stdout=%q stderr=%q", exitCode, stdout.String(), stderr.String())
	}
}

func TestLanguageDoesNotTranslateJSONContract(t *testing.T) {
	exitCode, stdout, stderr := run(t, false, "--language=es", "version", "--json")
	if exitCode != int(cli.ExitOK) || stderr != "" || stdout != "{\"ok\":true,\"data\":{\"version\":\"1.2.3\"}}\n" {
		t.Fatalf("exit=%d stdout=%q stderr=%q", exitCode, stdout, stderr)
	}
}

func TestInvalidExplicitLanguageIsUsageError(t *testing.T) {
	exitCode, stdout, stderr := run(t, false, "--language=pt", "version", "--json")
	if exitCode != int(cli.ExitUsage) || stdout != "" || !strings.Contains(stderr, `"code":"usage_error"`) || !strings.Contains(stderr, "use en or es") {
		t.Fatalf("exit=%d stdout=%q stderr=%q", exitCode, stdout, stderr)
	}
}

func TestHelpCommandShowsRootHelp(t *testing.T) {
	exitCode, stdout, stderr := run(t, false, "help")

	if exitCode != int(cli.ExitOK) || stderr != "" || !strings.Contains(stdout, "DAIKU") || !strings.Contains(stdout, "completion") {
		t.Fatalf("exit=%d stdout=%q stderr=%q", exitCode, stdout, stderr)
	}
}

func TestHelpCommandShowsVersionHelpAsJSON(t *testing.T) {
	exitCode, stdout, stderr := run(t, false, "help", "version", "--json=TRUE")

	if exitCode != int(cli.ExitOK) || stderr != "" || !json.Valid([]byte(stdout)) {
		t.Fatalf("exit=%d stdout=%q stderr=%q", exitCode, stdout, stderr)
	}
	if !strings.Contains(stdout, `"command":"daiku version"`) || !strings.Contains(stdout, `"metadata":{"name":"version","path":"daiku version"`) || !strings.Contains(stdout, "Print the Daiku CLI version") {
		t.Fatalf("unexpected help envelope: %q", stdout)
	}
	if !strings.Contains(stdout, `"command":"daiku help version --agent"`) {
		t.Fatalf("help breadcrumbs do not target version: %q", stdout)
	}
}

func TestUnknownHelpTopicIsJSONUsageError(t *testing.T) {
	exitCode, stdout, stderr := run(t, false, "help", "not-a-topic", "--json")

	if exitCode != int(cli.ExitUsage) || stdout != "" || !json.Valid([]byte(stderr)) || !strings.Contains(stderr, "usage_error") {
		t.Fatalf("exit=%d stdout=%q stderr=%q", exitCode, stdout, stderr)
	}
}

func TestCompletionCommandForBash(t *testing.T) {
	exitCode, stdout, stderr := run(t, false, "completion", "bash")

	if exitCode != int(cli.ExitOK) || stderr != "" || !strings.Contains(stdout, "bash completion") || !strings.Contains(stdout, "__daiku") {
		t.Fatalf("exit=%d stdout-prefix=%q stderr=%q", exitCode, firstBytes(stdout, 120), stderr)
	}
}

func TestGeneratedShellCompletionRequestReachesCobra(t *testing.T) {
	for _, request := range []string{cobra.ShellCompRequestCmd, cobra.ShellCompNoDescRequestCmd} {
		t.Run(request, func(t *testing.T) {
			exitCode, stdout, stderr := run(t, false, request, "version", "")

			if exitCode != int(cli.ExitOK) || strings.Contains(stderr, "unknown command") || !strings.HasPrefix(stdout, ":") {
				t.Fatalf("exit=%d stdout=%q stderr=%q", exitCode, stdout, stderr)
			}
		})
	}
}

func firstBytes(value string, limit int) string {
	if len(value) <= limit {
		return value
	}
	return value[:limit]
}

func TestVersionHumanOutput(t *testing.T) {
	exitCode, stdout, stderr := run(t, false, "version")

	if exitCode != int(cli.ExitOK) || stdout != "1.2.3\n" || stderr != "" {
		t.Fatalf("got exit=%d stdout=%q stderr=%q", exitCode, stdout, stderr)
	}
}

func TestRootVersionFlagUsesConfiguredVersion(t *testing.T) {
	exitCode, stdout, stderr := run(t, false, "--version")

	if exitCode != int(cli.ExitOK) || stdout != "daiku version 1.2.3\n" || stderr != "" {
		t.Fatalf("got exit=%d stdout=%q stderr=%q", exitCode, stdout, stderr)
	}
}

func TestRootVersionFlagHonorsJSONModeInEitherPosition(t *testing.T) {
	for _, args := range [][]string{{"--version", "--json"}, {"--json", "--version"}} {
		exitCode, stdout, stderr := run(t, false, args...)
		want := "{\"ok\":true,\"data\":{\"version\":\"1.2.3\"}}\n"
		if exitCode != int(cli.ExitOK) || stdout != want || stderr != "" {
			t.Fatalf("args=%v exit=%d stdout=%q stderr=%q", args, exitCode, stdout, stderr)
		}
	}
}

func TestVersionJSONSuccessEnvelope(t *testing.T) {
	exitCode, stdout, stderr := run(t, false, "version", "--json")

	if exitCode != int(cli.ExitOK) || stderr != "" {
		t.Fatalf("got exit=%d stderr=%q", exitCode, stderr)
	}
	var envelope struct {
		OK   bool `json:"ok"`
		Data struct {
			Version string `json:"version"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &envelope); err != nil {
		t.Fatalf("invalid JSON %q: %v", stdout, err)
	}
	if !envelope.OK || envelope.Data.Version != "1.2.3" {
		t.Fatalf("unexpected envelope: %+v", envelope)
	}
}

func TestJSONBooleanSyntaxAndFlagPositions(t *testing.T) {
	tests := [][]string{
		{"--json=TRUE", "version"},
		{"version", "--json=TRUE"},
	}
	for _, args := range tests {
		exitCode, stdout, stderr := run(t, false, args...)
		if exitCode != int(cli.ExitOK) || stderr != "" || !json.Valid([]byte(stdout)) {
			t.Errorf("args=%v exit=%d stdout=%q stderr=%q", args, exitCode, stdout, stderr)
		}
	}
}

func TestUnknownCommandHasUsageExitAndHumanError(t *testing.T) {
	exitCode, stdout, stderr := run(t, false, "does-not-exist")

	if exitCode != int(cli.ExitUsage) || stdout != "" {
		t.Fatalf("got exit=%d stdout=%q", exitCode, stdout)
	}
	if !strings.HasPrefix(stderr, "Error: unknown command") {
		t.Fatalf("unexpected stderr: %q", stderr)
	}
}

func TestUnknownCommandJSONErrorEnvelope(t *testing.T) {
	exitCode, stdout, stderr := run(t, false, "does-not-exist", "--json=TRUE")

	if exitCode != int(cli.ExitUsage) || stdout != "" {
		t.Fatalf("got exit=%d stdout=%q", exitCode, stdout)
	}
	var envelope struct {
		OK    bool `json:"ok"`
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal([]byte(stderr), &envelope); err != nil {
		t.Fatalf("invalid JSON %q: %v", stderr, err)
	}
	if envelope.OK || envelope.Error.Code != "usage_error" || !strings.Contains(envelope.Error.Message, "unknown command") {
		t.Fatalf("unexpected envelope: %+v", envelope)
	}
}

func TestJSONErrorSupportsFlagBeforeUnknownCommand(t *testing.T) {
	exitCode, stdout, stderr := run(t, false, "--json=TRUE", "does-not-exist")

	if exitCode != int(cli.ExitUsage) || stdout != "" || !json.Valid([]byte(stderr)) {
		t.Fatalf("got exit=%d stdout=%q stderr=%q", exitCode, stdout, stderr)
	}
}

func TestInvalidBooleanFlagIsUsageError(t *testing.T) {
	exitCode, stdout, stderr := run(t, false, "version", "--json=foo")

	if exitCode != int(cli.ExitUsage) || stdout != "" {
		t.Fatalf("got exit=%d stdout=%q", exitCode, stdout)
	}
	if !strings.HasPrefix(stderr, "Error: invalid argument") {
		t.Fatalf("unexpected stderr: %q", stderr)
	}
}

func TestUnknownFlagIsUsageError(t *testing.T) {
	exitCode, _, stderr := run(t, false, "version", "--not-a-flag")

	if exitCode != int(cli.ExitUsage) || !strings.HasPrefix(stderr, "Error: unknown flag") {
		t.Fatalf("got exit=%d stderr=%q", exitCode, stderr)
	}
}

func TestInvalidPositionalInputIsUsageError(t *testing.T) {
	exitCode, _, stderr := run(t, false, "version", "extra")

	if exitCode != int(cli.ExitUsage) || !strings.HasPrefix(stderr, "Error: unknown command") {
		t.Fatalf("got exit=%d stderr=%q", exitCode, stderr)
	}
}

func TestMissingRequiredFlagIsUsageError(t *testing.T) {
	exitCode, stdout, stderr := run(t, false, "required", "--json")

	if exitCode != int(cli.ExitUsage) || stdout != "" || !json.Valid([]byte(stderr)) {
		t.Fatalf("got exit=%d stdout=%q stderr=%q", exitCode, stdout, stderr)
	}
	if strings.Contains(stderr, "internal_error") || !strings.Contains(stderr, "usage_error") {
		t.Fatalf("unexpected stderr: %q", stderr)
	}
}

func TestTypedErrorPreservesJSONCodeDetailsAndExitCode(t *testing.T) {
	exitCode, stdout, stderr := run(t, false, "fail", "--json")

	if exitCode != int(cli.ExitAuth) || stdout != "" {
		t.Fatalf("got exit=%d stdout=%q", exitCode, stdout)
	}
	var envelope struct {
		OK    bool `json:"ok"`
		Error struct {
			Code    string            `json:"code"`
			Message string            `json:"message"`
			Details map[string]string `json:"details"`
		} `json:"error"`
	}
	if err := json.Unmarshal([]byte(stderr), &envelope); err != nil {
		t.Fatalf("invalid JSON %q: %v", stderr, err)
	}
	if envelope.OK || envelope.Error.Code != "authentication_required" || envelope.Error.Message != "sign in first" || envelope.Error.Details["action"] != "daiku auth login" {
		t.Fatalf("unexpected envelope: %+v", envelope)
	}
}

func TestUntypedErrorIsMaskedInHumanAndJSONOutput(t *testing.T) {
	for _, args := range [][]string{{"internal-fail"}, {"internal-fail", "--json"}} {
		exitCode, stdout, stderr := run(t, false, args...)
		if exitCode != int(cli.ExitFailure) || stdout != "" {
			t.Fatalf("args=%v exit=%d stdout=%q", args, exitCode, stdout)
		}
		if strings.Contains(stderr, "database password") || !strings.Contains(stderr, "unexpected internal error") {
			t.Fatalf("args=%v leaked or unexpected stderr: %q", args, stderr)
		}
		if len(args) == 2 && !json.Valid([]byte(stderr)) {
			t.Fatalf("args=%v invalid JSON: %q", args, stderr)
		}
	}
}

type failingWriter struct{}

func (failingWriter) Write(_ []byte) (int, error) {
	return 0, errors.New("broken pipe")
}

func TestOutputWriteFailureIsMasked(t *testing.T) {
	var stderr bytes.Buffer
	app := cli.New(
		cli.WithIO(strings.NewReader(""), failingWriter{}, &stderr),
		cli.WithModule(versioncommand.New("1.2.3")),
	)

	exitCode := app.Run([]string{"version"})
	if exitCode != int(cli.ExitFailure) || stderr.String() != "Error: an unexpected internal error occurred\n" {
		t.Fatalf("exit=%d stderr=%q", exitCode, stderr.String())
	}
}

func TestHumanHelpWriteFailureIsMasked(t *testing.T) {
	var stderr bytes.Buffer
	app := cli.New(
		cli.WithIO(strings.NewReader(""), failingWriter{}, &stderr),
		cli.WithModule(versioncommand.New("1.2.3")),
	)

	exitCode := app.Run([]string{"--help"})
	if exitCode != int(cli.ExitFailure) || stderr.String() != "Error: an unexpected internal error occurred\n" {
		t.Fatalf("exit=%d stderr=%q", exitCode, stderr.String())
	}
}

func TestJSONHelpWriteFailureIsMasked(t *testing.T) {
	var stderr bytes.Buffer
	app := cli.New(
		cli.WithIO(strings.NewReader(""), failingWriter{}, &stderr),
		cli.WithModule(versioncommand.New("1.2.3")),
	)

	exitCode := app.Run([]string{"--json", "--help"})
	if exitCode != int(cli.ExitFailure) || !json.Valid(stderr.Bytes()) || strings.Contains(stderr.String(), "broken pipe") || !strings.Contains(stderr.String(), "internal_error") {
		t.Fatalf("exit=%d stderr=%q", exitCode, stderr.String())
	}
}

func TestJSONHelpIsMachineReadable(t *testing.T) {
	for _, args := range [][]string{{"--json=TRUE", "--help"}, {"--help", "--json=TRUE"}} {
		exitCode, stdout, stderr := run(t, false, args...)
		if exitCode != int(cli.ExitOK) || stderr != "" {
			t.Fatalf("args=%v exit=%d stderr=%q", args, exitCode, stderr)
		}
		var envelope map[string]any
		if err := json.Unmarshal([]byte(stdout), &envelope); err != nil {
			t.Fatalf("args=%v invalid JSON %q: %v", args, stdout, err)
		}
		if envelope["ok"] != true {
			t.Fatalf("args=%v unexpected envelope: %#v", args, envelope)
		}
	}
}

func TestCommandsJSONDiscoversCompleteCobraTreeOnce(t *testing.T) {
	exitCode, stdout, stderr := run(t, false, "commands", "--json")
	if exitCode != int(cli.ExitOK) || stderr != "" {
		t.Fatalf("exit=%d stdout=%q stderr=%q", exitCode, stdout, stderr)
	}
	var envelope struct {
		OK   bool `json:"ok"`
		Data struct {
			Commands []agent.Command `json:"commands"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &envelope); err != nil {
		t.Fatalf("invalid JSON %q: %v", stdout, err)
	}
	seen := map[string]bool{}
	for _, command := range envelope.Data.Commands {
		if seen[command.Path] {
			t.Fatalf("duplicate command path %q", command.Path)
		}
		seen[command.Path] = true
		if command.Path == "daiku" {
			for _, flag := range command.Flags {
				if flag.Name == "json" && flag.Default != "false" {
					t.Errorf("json default depends on introspection invocation: %+v", flag)
				}
			}
		}
	}
	for _, path := range []string{"daiku", "daiku commands", "daiku completion", "daiku help", "daiku version"} {
		if !seen[path] {
			t.Errorf("missing command %q", path)
		}
	}
}

func TestAgentModeImpliesJSONNoInputAndBreadcrumbs(t *testing.T) {
	exitCode, stdout, stderr := run(t, true, "version", "--agent", "--json=false")
	if exitCode != int(cli.ExitOK) || stderr != "" || strings.Contains(stdout, "\x1b[") {
		t.Fatalf("exit=%d stdout=%q stderr=%q", exitCode, stdout, stderr)
	}
	var envelope struct {
		OK          bool               `json:"ok"`
		Breadcrumbs []agent.Breadcrumb `json:"breadcrumbs"`
		Data        struct {
			Version string `json:"version"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &envelope); err != nil {
		t.Fatalf("invalid JSON %q: %v", stdout, err)
	}
	if !envelope.OK || envelope.Data.Version != "1.2.3" || len(envelope.Breadcrumbs) == 0 {
		t.Fatalf("unexpected envelope: %+v", envelope)
	}
}

type agentSafetyModule struct{ executed *bool }

func (module agentSafetyModule) Register(root *cobra.Command) {
	contextCommand := &cobra.Command{Use: "agent-context", RunE: func(command *cobra.Command, _ []string) error {
		human := cli.Human(command)
		return cli.WriteSuccess(command.OutOrStdout(), map[string]bool{
			"agent": human.Agent, "interactive": human.Interactive, "no_color": human.NoColor, "no_input": human.NoInput,
		})
	}}
	rawCommand := &cobra.Command{Use: "agent-raw", RunE: func(command *cobra.Command, _ []string) error {
		_, err := io.WriteString(command.OutOrStdout(), "raw output\n")
		return err
	}}
	readCommand := &cobra.Command{Use: "agent-read", RunE: func(command *cobra.Command, _ []string) error {
		buffer := make([]byte, 1)
		count, err := command.InOrStdin().Read(buffer)
		return cli.WriteSuccess(command.OutOrStdout(), map[string]any{"bytes_read": count, "eof": errors.Is(err, io.EOF)})
	}}
	largeNumberCommand := &cobra.Command{Use: "agent-large-number", RunE: func(command *cobra.Command, _ []string) error {
		_, err := io.WriteString(command.OutOrStdout(), `{"ok":true,"data":{"value":9007199254740993}}`+"\n")
		return err
	}}
	interactiveCommand := &cobra.Command{Use: "agent-interactive", RunE: func(*cobra.Command, []string) error {
		*module.executed = true
		return nil
	}}
	agent.MarkRequiresInput(interactiveCommand)
	root.AddCommand(contextCommand, rawCommand, readCommand, largeNumberCommand, interactiveCommand)
}

func agentSafetyApp(t *testing.T, executed *bool, args ...string) (int, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	app := cli.New(
		cli.WithIO(strings.NewReader("unexpected input"), &stdout, &stderr),
		cli.WithTerminalDetector(func(io.Writer) bool { return true }),
		cli.WithInteractiveDetector(func(io.Reader, io.Writer) bool { return true }),
		cli.WithEnvironment(func(string) (string, bool) { return "", false }),
		cli.WithModule(agentSafetyModule{executed: executed}),
	)
	return app.Run(args), stdout.String(), stderr.String()
}

func TestAgentContextCannotBeInteractive(t *testing.T) {
	executed := false
	exitCode, stdout, stderr := agentSafetyApp(t, &executed, "agent-context", "--agent")
	if exitCode != int(cli.ExitOK) || stderr != "" || !strings.Contains(stdout, `"agent":true`) ||
		!strings.Contains(stdout, `"interactive":false`) || !strings.Contains(stdout, `"no_color":true`) || !strings.Contains(stdout, `"no_input":true`) {
		t.Fatalf("exit=%d stdout=%q stderr=%q", exitCode, stdout, stderr)
	}
}

func TestAgentModeRejectsInteractiveCommandBeforeExecution(t *testing.T) {
	executed := false
	exitCode, stdout, stderr := agentSafetyApp(t, &executed, "agent-interactive", "--agent")
	if exitCode != int(cli.ExitUsage) || stdout != "" || executed || !strings.Contains(stderr, `"code":"interaction_required"`) || !strings.Contains(stderr, `"breadcrumbs"`) {
		t.Fatalf("exit=%d executed=%t stdout=%q stderr=%q", exitCode, executed, stdout, stderr)
	}
}

func TestNoInputRejectsInteractiveCommandWithoutEnablingAgentOutput(t *testing.T) {
	executed := false
	exitCode, stdout, stderr := agentSafetyApp(t, &executed, "agent-interactive", "--no-input", "--json")
	if exitCode != int(cli.ExitUsage) || stdout != "" || executed || !strings.Contains(stderr, `"code":"interaction_required"`) || strings.Contains(stderr, `"breadcrumbs"`) {
		t.Fatalf("exit=%d executed=%t stdout=%q stderr=%q", exitCode, executed, stdout, stderr)
	}
}

func TestAgentModeFailsClosedOnNonEnvelopeOutput(t *testing.T) {
	executed := false
	exitCode, stdout, stderr := agentSafetyApp(t, &executed, "agent-raw", "--agent")
	if exitCode != int(cli.ExitFailure) || stdout != "" || !strings.Contains(stderr, `"code":"agent_output_invalid"`) || strings.Contains(stderr, "raw output") {
		t.Fatalf("exit=%d stdout=%q stderr=%q", exitCode, stdout, stderr)
	}
}

type panicReader struct{}

func (panicReader) Read([]byte) (int, error) { panic("real stdin was read") }

func TestNoInputPhysicallyDisconnectsStdin(t *testing.T) {
	var stdout, stderr bytes.Buffer
	executed := false
	app := cli.New(
		cli.WithIO(panicReader{}, &stdout, &stderr),
		cli.WithEnvironment(func(string) (string, bool) { return "", false }),
		cli.WithModule(agentSafetyModule{executed: &executed}),
	)
	exitCode := app.Run([]string{"agent-read", "--no-input", "--json"})
	if exitCode != int(cli.ExitOK) || stderr.Len() != 0 || !strings.Contains(stdout.String(), `"bytes_read":0`) || !strings.Contains(stdout.String(), `"eof":true`) {
		t.Fatalf("exit=%d stdout=%q stderr=%q", exitCode, stdout.String(), stderr.String())
	}
}

func TestAgentModePreservesLargeJSONNumbers(t *testing.T) {
	executed := false
	exitCode, stdout, stderr := agentSafetyApp(t, &executed, "agent-large-number", "--agent")
	if exitCode != int(cli.ExitOK) || stderr != "" || !strings.Contains(stdout, `"value":9007199254740993`) {
		t.Fatalf("exit=%d stdout=%q stderr=%q", exitCode, stdout, stderr)
	}
}

func TestUnknownCommandInAgentModeHasMachineBreadcrumbs(t *testing.T) {
	exitCode, stdout, stderr := run(t, false, "does-not-exist", "--agent")
	if exitCode != int(cli.ExitUsage) || stdout != "" || !json.Valid([]byte(stderr)) || !strings.Contains(stderr, `"code":"usage_error"`) || !strings.Contains(stderr, `"breadcrumbs"`) {
		t.Fatalf("exit=%d stdout=%q stderr=%q", exitCode, stdout, stderr)
	}
}
