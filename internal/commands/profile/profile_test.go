package profile_test

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/DaikuFi/daiku-cli/internal/cli"
	profilecommand "github.com/DaikuFi/daiku-cli/internal/commands/profile"
	"github.com/DaikuFi/daiku-cli/internal/credentials"
	"github.com/DaikuFi/daiku-cli/internal/profiles"
)

type noCredentials struct{}

func (noCredentials) Get(string) (credentials.Token, error) {
	return credentials.Token{}, credentials.ErrNotFound
}
func (noCredentials) Put(string, credentials.Token) error { return nil }
func (noCredentials) Delete(string) error                 { return nil }

func profileApp(t *testing.T, cfg profiles.Config, terminal, interactive bool, width int, input string) (*cli.App, profiles.Store, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	store := profiles.Store{Path: filepath.Join(dir, "config.json")}
	if err := store.Save(cfg); err != nil {
		t.Fatal(err)
	}
	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	app := cli.New(
		cli.WithIO(strings.NewReader(input), stdout, stderr),
		cli.WithEnvironment(func(string) (string, bool) { return "", false }),
		cli.WithTerminalDetector(func(io.Writer) bool { return terminal }),
		cli.WithInteractiveDetector(func(io.Reader, io.Writer) bool { return interactive }),
		cli.WithTerminalWidthDetector(func(io.Writer) int { return width }),
		cli.WithModule(profilecommand.New(store, noCredentials{})),
	)
	return app, store, stdout, stderr
}

func testConfig() profiles.Config {
	return profiles.Config{Current: "cafe", Profiles: map[string]profiles.Profile{
		"cafe": {APIURL: "https://api.daiku.app/api/v1/"},
		"casa": {APIURL: "https://example.com/api/v1/"},
	}}
}

func TestListWideEnglishAndNarrowSpanishGoldens(t *testing.T) {
	app, _, stdout, stderr := profileApp(t, testConfig(), false, false, 100, "")
	if exit := app.Run([]string{"profile", "list", "--language=en"}); exit != 0 || stderr.Len() != 0 {
		t.Fatalf("exit=%d stderr=%q", exit, stderr.String())
	}
	wantWide := "NAME  API URL                        CURRENT\n" +
		"cafe  https://api.daiku.app/api/v1/  yes    \n" +
		"casa  https://example.com/api/v1/    no     \n"
	if stdout.String() != wantWide {
		t.Fatalf("wide=%q want=%q", stdout.String(), wantWide)
	}

	app, _, stdout, stderr = profileApp(t, testConfig(), true, true, 24, "")
	if exit := app.Run([]string{"profile", "list", "--language=es"}); exit != 0 || stderr.Len() != 0 {
		t.Fatalf("exit=%d stderr=%q", exit, stderr.String())
	}
	wantNarrow := "\x1b[1m\x1b[36mNOMBRE\x1b[0m: cafe\n\x1b[1m\x1b[36mURL DE API\x1b[0m: https://api.daiku.app/api/v1/\n\x1b[1m\x1b[36mACTUAL\x1b[0m: sí\n\n" +
		"\x1b[1m\x1b[36mNOMBRE\x1b[0m: casa\n\x1b[1m\x1b[36mURL DE API\x1b[0m: https://example.com/api/v1/\n\x1b[1m\x1b[36mACTUAL\x1b[0m: no\n"
	if stdout.String() != wantNarrow {
		t.Fatalf("narrow=%q want=%q", stdout.String(), wantNarrow)
	}
}

func TestListEmptyLocalized(t *testing.T) {
	app, _, stdout, _ := profileApp(t, profiles.Config{Profiles: map[string]profiles.Profile{}}, false, false, 80, "")
	if exit := app.Run([]string{"profile", "list", "--language=es"}); exit != 0 || stdout.String() != "No hay resultados.\n" {
		t.Fatalf("exit=%d stdout=%q", exit, stdout.String())
	}
}

func TestHumanErrorsAreLocalized(t *testing.T) {
	app, _, stdout, stderr := profileApp(t, testConfig(), false, false, 80, "")
	exit := app.Run([]string{"profile", "add", "casa", "--language=es"})
	if exit != int(cli.ExitConflict) || stdout.Len() != 0 || stderr.String() != "Error: el perfil ya existe\n" {
		t.Fatalf("exit=%d stdout=%q stderr=%q", exit, stdout.String(), stderr.String())
	}
}

func TestRemoveRequiresConfirmationForPipeAndKeepsJSONEnglish(t *testing.T) {
	app, store, stdout, stderr := profileApp(t, testConfig(), false, false, 80, "yes\n")
	exit := app.Run([]string{"profile", "remove", "casa", "--language=es", "--json"})
	if exit != int(cli.ExitUsage) || stdout.Len() != 0 || !json.Valid(stderr.Bytes()) || !strings.Contains(stderr.String(), `"code":"confirmation_required"`) || !strings.Contains(stderr.String(), "pass --yes") {
		t.Fatalf("exit=%d stdout=%q stderr=%q", exit, stdout.String(), stderr.String())
	}
	cfg, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := cfg.Profiles["casa"]; !ok {
		t.Fatal("profile removed without confirmation")
	}
}

func TestRemoveJSONNeverPromptsEvenOnTTY(t *testing.T) {
	app, _, stdout, stderr := profileApp(t, testConfig(), true, true, 80, "sí\n")
	exit := app.Run([]string{"profile", "remove", "casa", "--language=es", "--json"})
	if exit != int(cli.ExitUsage) || stdout.Len() != 0 || !json.Valid(stderr.Bytes()) || strings.Contains(stderr.String(), "Continuar") {
		t.Fatalf("exit=%d stdout=%q stderr=%q", exit, stdout.String(), stderr.String())
	}
}

func TestRemoveTTYSpanishAndPipeExplicitYes(t *testing.T) {
	app, store, stdout, stderr := profileApp(t, testConfig(), true, true, 80, "sí\n")
	if exit := app.Run([]string{"profile", "remove", "casa", "--language=es"}); exit != 0 {
		t.Fatalf("exit=%d stdout=%q stderr=%q", exit, stdout.String(), stderr.String())
	}
	if stdout.String() != "Perfil casa eliminado.\n" || !strings.Contains(stderr.String(), "Eliminar el perfil casa. ¿Continuar?") {
		t.Fatalf("stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
	cfg, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := cfg.Profiles["casa"]; ok {
		t.Fatal("profile was not removed")
	}

	app, _, stdout, stderr = profileApp(t, testConfig(), false, false, 80, "")
	if exit := app.Run([]string{"profile", "remove", "casa", "--yes", "--json"}); exit != 0 || stderr.Len() != 0 || stdout.String() != "{\"ok\":true,\"data\":{\"name\":\"casa\",\"removed\":true}}\n" {
		t.Fatalf("exit=%d stdout=%q stderr=%q", exit, stdout.String(), stderr.String())
	}
}
