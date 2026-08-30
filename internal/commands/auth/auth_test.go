package auth_test

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/DaikuFi/daiku-cli/internal/cli"
	authcommand "github.com/DaikuFi/daiku-cli/internal/commands/auth"
	"github.com/DaikuFi/daiku-cli/internal/credentials"
	"github.com/DaikuFi/daiku-cli/internal/profiles"
)

type memoryCredentials struct{ tokens map[string]credentials.Token }

func (m *memoryCredentials) Get(profile string) (credentials.Token, error) {
	token, ok := m.tokens[profile]
	if !ok {
		return token, credentials.ErrNotFound
	}
	return token, nil
}
func (m *memoryCredentials) Put(profile string, token credentials.Token) error {
	m.tokens[profile] = token
	return nil
}
func (m *memoryCredentials) Delete(profile string) error { delete(m.tokens, profile); return nil }

func authApp(t *testing.T, terminal, interactive bool, input string) (*cli.App, *memoryCredentials, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	store := profiles.Store{Path: filepath.Join(dir, "config.json")}
	if err := store.Save(profiles.Config{Current: "personal", Profiles: map[string]profiles.Profile{"personal": {APIURL: "https://api.daiku.app/api/v1/"}}}); err != nil {
		t.Fatal(err)
	}
	credentialsStore := &memoryCredentials{tokens: map[string]credentials.Token{"personal": {AccessToken: "secret", ExpiresAt: 1893456000, Scope: "finance:read"}}}
	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	app := cli.New(
		cli.WithIO(strings.NewReader(input), stdout, stderr),
		cli.WithEnvironment(func(string) (string, bool) { return "", false }),
		cli.WithTerminalDetector(func(io.Writer) bool { return terminal }),
		cli.WithInteractiveDetector(func(io.Reader, io.Writer) bool { return interactive }),
		cli.WithModule(authcommand.New(store, credentialsStore, nil)),
	)
	return app, credentialsStore, stdout, stderr
}

func TestStatusHumanIsLocalizedAndJSONStaysEnglish(t *testing.T) {
	app, _, stdout, stderr := authApp(t, false, false, "")
	if exit := app.Run([]string{"auth", "status", "--language=es"}); exit != 0 || stderr.Len() != 0 || stdout.String() != "El perfil personal tiene una sesión activa.\n" {
		t.Fatalf("exit=%d stdout=%q stderr=%q", exit, stdout.String(), stderr.String())
	}

	app, _, stdout, stderr = authApp(t, false, false, "")
	if exit := app.Run([]string{"auth", "status", "--language=es", "--json"}); exit != 0 || stderr.Len() != 0 || !json.Valid(stdout.Bytes()) || strings.Contains(stdout.String(), "sesión") || !strings.Contains(stdout.String(), `"logged_in":true`) {
		t.Fatalf("exit=%d stdout=%q stderr=%q", exit, stdout.String(), stderr.String())
	}
}

func TestStatusHelpDocumentsExitZeroForLoggedOutState(t *testing.T) {
	app, _, stdout, stderr := authApp(t, false, false, "")
	if exit := app.Run([]string{"auth", "status", "--help"}); exit != 0 || stderr.Len() != 0 {
		t.Fatalf("exit=%d stdout=%q stderr=%q", exit, stdout.String(), stderr.String())
	}
	for _, want := range []string{"Exit code 0", "logged_in is false", "data.logged_in", "--json"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("help missing %q: %s", want, stdout.String())
		}
	}
}

func TestLoginAgentAndNoInputFailBeforeStartingOAuth(t *testing.T) {
	for _, args := range [][]string{{"auth", "login", "--agent"}, {"auth", "login", "--no-input", "--json"}} {
		app, _, stdout, stderr := authApp(t, true, true, "unexpected input")
		exit := app.Run(args)
		if exit != int(cli.ExitUsage) || stdout.Len() != 0 || !strings.Contains(stderr.String(), `"code":"interaction_required"`) {
			t.Fatalf("args=%v exit=%d stdout=%q stderr=%q", args, exit, stdout.String(), stderr.String())
		}
	}
}

func TestStatusLoggedOutIsReadableStateWithExitZero(t *testing.T) {
	app, store, stdout, stderr := authApp(t, false, false, "")
	if err := store.Delete("personal"); err != nil {
		t.Fatal(err)
	}
	if exit := app.Run([]string{"auth", "status", "--json"}); exit != 0 || stderr.Len() != 0 || stdout.String() != "{\"ok\":true,\"data\":{\"logged_in\":false,\"profile\":\"personal\"}}\n" {
		t.Fatalf("stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
}

func TestLogoutRequiresExplicitConfirmationForPipe(t *testing.T) {
	app, store, stdout, stderr := authApp(t, false, false, "yes\n")
	exit := app.Run([]string{"auth", "logout", "--local-only", "--language=es", "--json"})
	if exit != int(cli.ExitUsage) || stdout.Len() != 0 || !strings.Contains(stderr.String(), `"code":"confirmation_required"`) || !strings.Contains(stderr.String(), "pass --yes") {
		t.Fatalf("exit=%d stdout=%q stderr=%q", exit, stdout.String(), stderr.String())
	}
	if _, err := store.Get("personal"); err != nil {
		t.Fatalf("credentials changed: %v", err)
	}
}

func TestLogoutTTYSpanishAndPipeExplicitYes(t *testing.T) {
	app, store, stdout, stderr := authApp(t, true, true, "sí\n")
	if exit := app.Run([]string{"auth", "logout", "--local-only", "--language=es"}); exit != 0 {
		t.Fatalf("exit=%d stdout=%q stderr=%q", exit, stdout.String(), stderr.String())
	}
	if stdout.String() != "Sesión cerrada para el perfil personal.\n" || !strings.Contains(stderr.String(), "Cerrar la sesión del perfil personal. ¿Continuar?") {
		t.Fatalf("stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
	if _, err := store.Get("personal"); err == nil {
		t.Fatal("credentials were not removed")
	}

	app, store, stdout, stderr = authApp(t, false, false, "")
	if exit := app.Run([]string{"auth", "logout", "--local-only", "--yes", "--json"}); exit != 0 || stderr.Len() != 0 || stdout.String() != "{\"ok\":true,\"data\":{\"logged_in\":false,\"profile\":\"personal\",\"revoked\":false}}\n" {
		t.Fatalf("exit=%d stdout=%q stderr=%q", exit, stdout.String(), stderr.String())
	}
	if _, err := store.Get("personal"); err == nil {
		t.Fatal("credentials were not removed")
	}
}
