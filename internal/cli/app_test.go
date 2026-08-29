package cli_test

import (
	"bytes"
	"encoding/json"
	"io"
	"strings"
	"testing"

	"github.com/DaikuFi/daiku-cli/internal/cli"
	versioncommand "github.com/DaikuFi/daiku-cli/internal/commands/version"
	"github.com/spf13/cobra"
)

func run(t *testing.T, terminal bool, args ...string) (int, string, string) {
	t.Helper()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	app := cli.New(
		cli.WithIO(strings.NewReader(""), &stdout, &stderr),
		cli.WithVersion("1.2.3"),
		cli.WithModule(versioncommand.New("1.2.3")),
		cli.WithModule(failureModule{}),
		cli.WithTerminalDetector(func(_ io.Writer) bool { return terminal }),
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

func TestVersionHumanOutput(t *testing.T) {
	exitCode, stdout, stderr := run(t, false, "version")

	if exitCode != int(cli.ExitOK) || stdout != "1.2.3\n" || stderr != "" {
		t.Fatalf("got exit=%d stdout=%q stderr=%q", exitCode, stdout, stderr)
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
	exitCode, stdout, stderr := run(t, false, "does-not-exist", "--json")

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

func TestJSONHelpIsMachineReadable(t *testing.T) {
	exitCode, stdout, stderr := run(t, false, "--json", "--help")

	if exitCode != int(cli.ExitOK) || stderr != "" {
		t.Fatalf("got exit=%d stderr=%q", exitCode, stderr)
	}
	var envelope map[string]any
	if err := json.Unmarshal([]byte(stdout), &envelope); err != nil {
		t.Fatalf("invalid JSON %q: %v", stdout, err)
	}
	if envelope["ok"] != true {
		t.Fatalf("unexpected envelope: %#v", envelope)
	}
}
