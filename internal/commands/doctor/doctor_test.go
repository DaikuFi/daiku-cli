package doctor

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/DaikuFi/daiku-cli/internal/agent"
	"github.com/DaikuFi/daiku-cli/internal/cli"
	"github.com/DaikuFi/daiku-cli/internal/credentials"
	"github.com/DaikuFi/daiku-cli/internal/profiles"
	"github.com/DaikuFi/daiku-cli/internal/skillmeta"
)

type fakeCredentials struct {
	token               credentials.Token
	err                 error
	gets, puts, deletes int
}

func (f *fakeCredentials) Get(string) (credentials.Token, error) { f.gets++; return f.token, f.err }
func (f *fakeCredentials) Put(string, credentials.Token) error   { f.puts++; return nil }
func (f *fakeCredentials) Delete(string) error                   { f.deletes++; return nil }

type resolverFunc func(context.Context, string) ([]string, error)

func (f resolverFunc) LookupHost(ctx context.Context, host string) ([]string, error) {
	return f(ctx, host)
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

type closerFunc func() error

func (f closerFunc) Close() error { return f() }

func testEnvironment(t *testing.T) (Environment, *fakeCredentials, *int) {
	t.Helper()
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	store := profiles.Store{Path: filepath.Join(dir, "config.json")}
	if err := store.Save(profiles.Config{Current: "work", Profiles: map[string]profiles.Profile{"work": {APIURL: "https://api.example.test/api/v1/"}}}); err != nil {
		t.Fatal(err)
	}
	creds := &fakeCredentials{token: credentials.Token{AccessToken: "access-secret", RefreshToken: "refresh-secret", ExpiresAt: 2_000_000_000, Scope: "finance:read finance:write"}}
	calls := 0
	env := Environment{
		Version: "v1.2.3", ProfileStore: store, Credentials: creds,
		Executable: func() (string, error) { return "/opt/daiku", nil },
		LookPath:   func(string) (string, error) { return "/opt/daiku", nil },
		Now:        func() time.Time { return time.Unix(1_900_000_000, 0) },
		Resolver:   resolverFunc(func(context.Context, string) ([]string, error) { return []string{"sensitive-ip"}, nil }),
		ProbeTransport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			calls++
			if r.Header.Get("Authorization") != "Bearer access-secret" {
				t.Fatal("missing auth header")
			}
			return &http.Response{StatusCode: 204, Body: io.NopCloser(strings.NewReader("secret body")), Header: make(http.Header)}, nil
		}),
		Commands: func() []agent.Command {
			return []agent.Command{{Name: "doctor", Path: "daiku doctor", Use: "daiku doctor", Runnable: true, ReadOnly: true, Aliases: []string{}, Flags: []agent.Flag{}, Subcommands: []agent.CommandSummary{}}}
		},
		MCPReady:       func(context.Context) error { return nil },
		ListenLoopback: func(context.Context) (io.Closer, error) { return closerFunc(func() error { return nil }), nil },
		LookupEnv:      func(string) (string, bool) { return "", false },
		UserHomeDir:    func() (string, error) { return dir, nil },
		SkillDigests:   skillmeta.Digests,
		SchemaCommit:   "commit", SchemaSHA256: "digest",
	}
	return env, creds, &calls
}

func runReport(t *testing.T, m Module, ctx context.Context) Report {
	t.Helper()
	report, err := m.run(ctx)
	if err != nil {
		t.Fatal(err)
	}
	return report
}

func find(report Report, code string) *Check {
	for i := range report.Checks {
		if report.Checks[i].Code == code {
			return &report.Checks[i]
		}
	}
	return nil
}

func TestHealthyCoreAndDeterministicOrder(t *testing.T) {
	env, creds, calls := testEnvironment(t)
	m, err := New(env)
	if err != nil {
		t.Fatal(err)
	}
	r1, r2 := runReport(t, m, context.Background()), runReport(t, m, context.Background())
	if len(r1.Checks) != 12 || len(r2.Checks) != 12 {
		t.Fatalf("checks=%d", len(r1.Checks))
	}
	for i := range r1.Checks {
		if r1.Checks[i].Code != r2.Checks[i].Code {
			t.Fatalf("order changed at %d", i)
		}
	}
	if *calls != 2 || creds.gets != 2 || creds.puts != 0 || creds.deletes != 0 {
		t.Fatalf("calls=%d credentials=%+v", *calls, creds)
	}
	if find(r1, "api_ok") == nil || find(r1, "schema_compatibility_unknown") == nil {
		t.Fatalf("report=%+v", r1)
	}
}

func TestCancellationStopsBeforeSensitiveOrBlockingChecks(t *testing.T) {
	env, creds, _ := testEnvironment(t)
	var dns, transport, skillFS, mcpCalls, listeners int
	env.Resolver = resolverFunc(func(context.Context, string) ([]string, error) { dns++; return nil, nil })
	env.ProbeTransport = roundTripFunc(func(*http.Request) (*http.Response, error) { transport++; return nil, errors.New("unexpected") })
	env.Lstat = func(string) (os.FileInfo, error) { skillFS++; return nil, os.ErrNotExist }
	env.ReadSkillFile = func(string) ([]byte, error) { skillFS++; return nil, errors.New("unexpected") }
	env.MCPReady = func(context.Context) error { mcpCalls++; return nil }
	env.ListenLoopback = func(context.Context) (io.Closer, error) {
		listeners++
		return closerFunc(func() error { return nil }), nil
	}
	m, _ := New(env)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := m.run(ctx); !isCancelled(err) {
		t.Fatalf("err=%v", err)
	}
	if creds.gets != 0 || dns != 0 || transport != 0 || skillFS != 0 || mcpCalls != 0 || listeners != 0 {
		t.Fatalf("calls credentials=%d dns=%d transport=%d skill=%d mcp=%d listener=%d", creds.gets, dns, transport, skillFS, mcpCalls, listeners)
	}

	var out, errOut bytes.Buffer
	app := cli.New(cli.WithContext(ctx), cli.WithIO(strings.NewReader(""), &out, &errOut), cli.WithModule(m))
	if code := app.Run([]string{"doctor", "--agent"}); code != int(cli.ExitConflict) || out.Len() != 0 || !strings.Contains(errOut.String(), `"code":"operation_cancelled"`) {
		t.Fatalf("code=%d out=%q err=%q", code, out.String(), errOut.String())
	}
}

func TestMidRunCancellationStopsAllLaterChecks(t *testing.T) {
	env, creds, _ := testEnvironment(t)
	ctx, cancel := context.WithCancel(context.Background())
	var dns, transport, skillFS, mcpCalls, listeners int
	env.LookPath = func(string) (string, error) { cancel(); return "/opt/daiku", nil }
	env.Resolver = resolverFunc(func(context.Context, string) ([]string, error) { dns++; return nil, nil })
	env.ProbeTransport = roundTripFunc(func(*http.Request) (*http.Response, error) { transport++; return nil, errors.New("unexpected") })
	env.Lstat = func(string) (os.FileInfo, error) { skillFS++; return nil, os.ErrNotExist }
	env.ReadSkillFile = func(string) ([]byte, error) { skillFS++; return nil, errors.New("unexpected") }
	env.MCPReady = func(context.Context) error { mcpCalls++; return nil }
	env.ListenLoopback = func(context.Context) (io.Closer, error) { listeners++; return nil, errors.New("unexpected") }
	m, _ := New(env)
	if _, err := m.run(ctx); !isCancelled(err) {
		t.Fatalf("err=%v", err)
	}
	if creds.gets != 0 || dns != 0 || transport != 0 || skillFS != 0 || mcpCalls != 0 || listeners != 0 {
		t.Fatalf("post-cancel calls credentials=%d dns=%d transport=%d skill=%d mcp=%d listener=%d", creds.gets, dns, transport, skillFS, mcpCalls, listeners)
	}
}

func TestCancellationDuringSkillReadMakesNoFurtherCalls(t *testing.T) {
	env, _, _ := testEnvironment(t)
	home := filepath.Join(t.TempDir(), "codex")
	installSkill(t, home)
	env.LookupEnv = func(string) (string, bool) { return home, true }
	ctx, cancel := context.WithCancel(context.Background())
	var lstats, reads, dns, transport, mcpCalls, listeners int
	env.Resolver = resolverFunc(func(context.Context, string) ([]string, error) { dns++; return []string{"private"}, nil })
	env.ProbeTransport = roundTripFunc(func(*http.Request) (*http.Response, error) {
		transport++
		return &http.Response{StatusCode: 204, Body: io.NopCloser(strings.NewReader("")), Header: make(http.Header)}, nil
	})
	realLstat := os.Lstat
	env.Lstat = func(path string) (os.FileInfo, error) {
		lstats++
		info, err := realLstat(path)
		cancel()
		return info, err
	}
	env.ReadSkillFile = func(string) ([]byte, error) { reads++; return nil, errors.New("unexpected") }
	env.MCPReady = func(context.Context) error { mcpCalls++; return nil }
	env.ListenLoopback = func(context.Context) (io.Closer, error) { listeners++; return nil, errors.New("unexpected") }
	m, _ := New(env)
	if _, err := m.run(ctx); !isCancelled(err) {
		t.Fatalf("err=%v", err)
	}
	if lstats != 1 || reads != 0 || dns != 1 || transport != 1 || mcpCalls != 0 || listeners != 0 {
		t.Fatalf("lstat=%d reads=%d dns=%d transport=%d mcp=%d listener=%d", lstats, reads, dns, transport, mcpCalls, listeners)
	}
}

func isCancelled(err error) bool {
	var cliErr *cli.Error
	return errors.As(err, &cliErr) && cliErr.Code == "operation_cancelled" && cliErr.ExitCode == cli.ExitConflict
}

func TestProbeUsesExactlyOneRoundTripAndAuthorizationTransmission(t *testing.T) {
	env, _, _ := testEnvironment(t)
	var calls, auth int
	env.ProbeTransport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		if r.Header.Get("Authorization") != "" {
			auth++
		}
		return nil, errors.New("stale connection; caller must not retry")
	})
	m, _ := New(env)
	report := runReport(t, m, context.Background())
	if find(report, "api_unreachable") == nil || calls != 1 || auth != 1 {
		t.Fatalf("calls=%d auth=%d report=%+v", calls, auth, report)
	}
}

func TestProbeStatusAndNetworkClassification(t *testing.T) {
	tests := []struct {
		name, code string
		status     int
		err        error
	}{
		{"offline", "api_unreachable", 0, errors.New("offline")}, {"tls", "api_unreachable", 0, errors.New("tls failure")},
		{"401", "api_unauthorized", 401, nil}, {"403", "api_forbidden", 403, nil}, {"429", "api_rate_limited", 429, nil}, {"500", "api_server_error", 500, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env, _, calls := testEnvironment(t)
			env.ProbeTransport = roundTripFunc(func(*http.Request) (*http.Response, error) {
				*calls++
				if tt.err != nil {
					return nil, tt.err
				}
				return &http.Response{StatusCode: tt.status, Body: io.NopCloser(strings.NewReader("private")), Header: make(http.Header)}, nil
			})
			m, _ := New(env)
			report := runReport(t, m, context.Background())
			if find(report, tt.code) == nil {
				t.Fatalf("want %s: %+v", tt.code, report)
			}
			if *calls != 1 {
				t.Fatalf("calls=%d", *calls)
			}
		})
	}
}

func TestDNSFailureAndTimeout(t *testing.T) {
	env, _, calls := testEnvironment(t)
	env.Resolver = resolverFunc(func(context.Context, string) ([]string, error) { return nil, errors.New("dns leaked detail") })
	m, _ := New(env)
	report := runReport(t, m, context.Background())
	if find(report, "dns_failed") == nil || find(report, "api_probe_skipped") == nil || *calls != 0 {
		t.Fatalf("report=%+v calls=%d", report, *calls)
	}

	env, _, _ = testEnvironment(t)
	env.ProbeTimeout = time.Millisecond
	env.ProbeTransport = roundTripFunc(func(r *http.Request) (*http.Response, error) { <-r.Context().Done(); return nil, r.Context().Err() })
	m, _ = New(env)
	if find(runReport(t, m, context.Background()), "api_timeout") == nil {
		t.Fatal("timeout not classified")
	}
}

func TestExpiredMissingIncompleteAndScopeNeverProbeOrWrite(t *testing.T) {
	tests := []struct {
		name, code string
		token      credentials.Token
		err        error
	}{
		{"missing", "credentials_missing", credentials.Token{}, credentials.ErrNotFound},
		{"incomplete", "credentials_incomplete", credentials.Token{AccessToken: "secret"}, nil},
		{"expired", "token_expired", credentials.Token{AccessToken: "secret", RefreshToken: "refresh", ExpiresAt: 1, Scope: "finance:read"}, nil},
		{"scope", "token_scope_incomplete", credentials.Token{AccessToken: "secret", RefreshToken: "refresh", ExpiresAt: 2_000_000_000, Scope: "finance:write"}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env, creds, calls := testEnvironment(t)
			creds.token, creds.err = tt.token, tt.err
			m, _ := New(env)
			report := runReport(t, m, context.Background())
			if find(report, tt.code) == nil {
				t.Fatalf("%+v", report)
			}
			if *calls != 0 || creds.puts != 0 || creds.deletes != 0 {
				t.Fatalf("network/write occurred")
			}
		})
	}
}

func TestProfileCorruptAndUnsafe(t *testing.T) {
	for _, mode := range []string{"corrupt", "unsafe"} {
		t.Run(mode, func(t *testing.T) {
			env, _, calls := testEnvironment(t)
			if mode == "corrupt" {
				if err := os.WriteFile(env.ProfileStore.Path, []byte("{"), 0o600); err != nil {
					t.Fatal(err)
				}
			} else {
				if err := os.Chmod(env.ProfileStore.Path, 0o644); err != nil {
					t.Fatal(err)
				}
			}
			m, _ := New(env)
			if find(runReport(t, m, context.Background()), "profile_invalid") == nil {
				t.Fatal("invalid profile not found")
			}
			if *calls != 0 {
				t.Fatal("unsafe profile caused network")
			}
		})
	}
}

func TestVersions(t *testing.T) {
	for _, tt := range []struct{ v, code string }{{"dev", "version_development"}, {"old", "version_malformed"}, {"v1.x", "version_malformed"}, {"1.2.3", "version_ok"}} {
		if got := versionCheck(tt.v).Code; got != tt.code {
			t.Fatalf("%q=%s", tt.v, got)
		}
	}
}

func TestSkillsAbsentCorruptStaleAndCurrent(t *testing.T) {
	t.Run("defaults and overrides", func(t *testing.T) {
		env, _, _ := testEnvironment(t)
		userHome, _ := env.UserHomeDir()
		installSkill(t, filepath.Join(userHome, ".codex"))
		installSkill(t, filepath.Join(userHome, ".claude"))
		env.Commands = commandsFromInstalledSkill(t, filepath.Join(userHome, ".codex"))
		m, _ := New(env)
		for _, item := range []struct{ name, variable, code string }{{"codex", "CODEX_HOME", "codex_skill_ok"}, {"claude", "CLAUDE_HOME", "claude_skill_ok"}} {
			check, err := m.skill(context.Background(), item.name, item.variable)
			if err != nil || check.Code != item.code {
				t.Fatalf("%s: check=%+v err=%v", item.name, check, err)
			}
		}
		override := filepath.Join(t.TempDir(), "agent")
		installSkill(t, override)
		m.env.LookupEnv = func(name string) (string, bool) {
			if name == "CODEX_HOME" {
				return override, true
			}
			return "", false
		}
		check, err := m.skill(context.Background(), "codex", "CODEX_HOME")
		if err != nil || check.Code != "codex_skill_ok" || strings.Contains(check.Message+check.Hint, override) {
			t.Fatalf("override leaked or failed: %+v %v", check, err)
		}
		m.env.Commands = func() []agent.Command { return nil }
		check, err = m.skill(context.Background(), "codex", "CODEX_HOME")
		if err != nil || check.Code != "codex_skill_stale" {
			t.Fatalf("live metadata mismatch=%+v err=%v", check, err)
		}
		m.env.LookupEnv = func(string) (string, bool) { return "relative/home", true }
		check, _ = m.skill(context.Background(), "codex", "CODEX_HOME")
		if check.Code != "codex_home_invalid" || strings.Contains(check.Message+check.Hint, "relative") {
			t.Fatalf("relative override=%+v", check)
		}
	})

	mutations := []struct {
		name, want string
		mutate     func(*testing.T, string)
	}{
		{"arbitrary skill", "codex_skill_corrupt", func(t *testing.T, root string) {
			writeFile(t, filepath.Join(root, "SKILL.md"), []byte("arbitrary"), 0o644)
		}},
		{"stale reference", "codex_skill_stale", func(t *testing.T, root string) {
			writeFile(t, filepath.Join(root, "references", "commands.json"), []byte(`{"commands":[]}`), 0o644)
		}},
		{"symlink", "codex_skill_corrupt", func(t *testing.T, root string) {
			path := filepath.Join(root, "SKILL.md")
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(filepath.Join(root, "agents", "openai.yaml"), path); err != nil {
				t.Fatal(err)
			}
		}},
		{"fifo", "codex_skill_corrupt", func(t *testing.T, root string) {
			path := filepath.Join(root, "SKILL.md")
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
			if err := syscall.Mkfifo(path, 0o644); err != nil {
				t.Fatal(err)
			}
		}},
		{"unsafe permissions", "codex_skill_corrupt", func(t *testing.T, root string) {
			if err := os.Chmod(filepath.Join(root, "SKILL.md"), 0o666); err != nil {
				t.Fatal(err)
			}
		}},
		{"oversized", "codex_skill_corrupt", func(t *testing.T, root string) {
			writeFile(t, filepath.Join(root, "SKILL.md"), bytes.Repeat([]byte("x"), int(maxSkillFileSize)+1), 0o644)
		}},
		{"missing", "codex_skill_corrupt", func(t *testing.T, root string) {
			if err := os.Remove(filepath.Join(root, "SKILL.md")); err != nil {
				t.Fatal(err)
			}
		}},
		{"malformed integrity", "codex_skill_corrupt", func(t *testing.T, root string) {
			writeFile(t, filepath.Join(root, "integrity.json"), []byte("{"), 0o644)
		}},
		{"unsafe directory", "codex_skill_unsafe", func(t *testing.T, root string) {
			if err := os.Chmod(filepath.Join(root, "references"), 0o777); err != nil {
				t.Fatal(err)
			}
		}},
	}
	for _, tt := range mutations {
		t.Run(tt.name, func(t *testing.T) {
			env, _, _ := testEnvironment(t)
			home := filepath.Join(t.TempDir(), "codex")
			root := installSkill(t, home)
			tt.mutate(t, root)
			env.LookupEnv = func(string) (string, bool) { return home, true }
			m, _ := New(env)
			check, err := m.skill(context.Background(), "codex", "CODEX_HOME")
			if err != nil || check.Code != tt.want {
				t.Fatalf("check=%+v err=%v", check, err)
			}
		})
	}
}

func installSkill(t *testing.T, home string) string {
	t.Helper()
	root := filepath.Join(home, "skills", "daiku")
	for _, dir := range []string{home, filepath.Join(home, "skills"), root, filepath.Join(root, "agents"), filepath.Join(root, "references")} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for _, path := range []string{
		"SKILL.md",
		"integrity.json",
		"agents/openai.yaml",
		"references/commands.json",
		"references/commands.md",
		"references/safety.md",
		"references/workflows.md",
	} {
		raw, err := os.ReadFile(filepath.Join("..", "..", "..", "skills", "daiku", filepath.FromSlash(path)))
		if err != nil {
			t.Fatal(err)
		}
		writeFile(t, filepath.Join(root, filepath.FromSlash(path)), raw, 0o644)
	}
	return root
}

func writeFile(t *testing.T, path string, data []byte, mode os.FileMode) {
	t.Helper()
	if err := os.WriteFile(path, data, mode); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, mode); err != nil {
		t.Fatal(err)
	}
}

func commandsFromInstalledSkill(t *testing.T, home string) func() []agent.Command {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(home, "skills", "daiku", "references", "commands.json"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Commands []agent.Command `json:"commands"`
	}
	if err := json.Unmarshal(raw, &manifest); err != nil {
		t.Fatal(err)
	}
	return func() []agent.Command { return manifest.Commands }
}

func TestMCPFailureCancellationAndBlockedLoopback(t *testing.T) {
	env, _, _ := testEnvironment(t)
	env.MCPReady = func(context.Context) error { return errors.New("secret provider error") }
	m, _ := New(env)
	if find(runReport(t, m, context.Background()), "mcp_failed") == nil {
		t.Fatal("mcp failure")
	}
	env.MCPReady = func(ctx context.Context) error { <-ctx.Done(); return ctx.Err() }
	env.ProbeTimeout = time.Millisecond
	m, _ = New(env)
	if find(runReport(t, m, context.Background()), "mcp_cancelled") == nil {
		t.Fatal("mcp cancellation not reported")
	}
	env, _, _ = testEnvironment(t)
	env.ListenLoopback = func(context.Context) (io.Closer, error) { return nil, errors.New("address and user") }
	m, _ = New(env)
	if find(runReport(t, m, context.Background()), "oauth_callback_blocked") == nil {
		t.Fatal("loopback")
	}
}

func TestHumanEnglishSpanishAgentJSONUsageAndRenderFailure(t *testing.T) {
	report := Report{Status: Fail, Checks: []Check{
		{Code: "installation_path_ok", Status: Pass, Message: "the running executable matches PATH"},
		{Code: "token_expired", Status: Fail, Message: "the access token is expired and was not sent", Hint: "run daiku auth login"},
		{Code: "api_rate_limited", Status: Warning, Message: "the API rate limited the single probe", Hint: "wait before running diagnostics again"},
		{Code: "api_forbidden", Status: Fail, Message: "the API denied the authenticated probe", Hint: "check the selected profile permissions"},
		{Code: "api_unreachable", Status: Fail, Message: "the API probe failed before receiving a response", Hint: "check TLS and network connectivity"},
		{Code: "schema_compatibility_unknown", Status: Warning, Message: "local API schema metadata is compiled in, but the server provided no compatibility signal", Hint: "compatibility cannot be inferred from reachability alone"},
	}, Summary: Summary{Passed: 1, Warnings: 2, Failed: 3}}
	wants := map[string][]string{
		"en": {"PASS", "WARN", "FAIL", "Hint: run daiku auth login", "rate limited", "selected profile permissions", "TLS and network", "compatibility cannot be inferred"},
		"es": {"BIEN", "AVISO", "FALLO", "Sugerencia: ejecuta daiku auth login", "limitó la única comprobación", "permisos del perfil seleccionado", "TLS y la conectividad", "compatibilidad no se puede inferir"},
	}
	for _, lang := range []string{"en", "es"} {
		var b bytes.Buffer
		if err := renderHuman(&b, report, lang); err != nil {
			t.Fatal(err)
		}
		for _, want := range wants[lang] {
			if !strings.Contains(b.String(), want) {
				t.Fatalf("%s missing %q: %q", lang, want, b.String())
			}
		}
	}
	if err := renderHuman(failingWriter{}, report, "en"); err == nil {
		t.Fatal("render failure succeeded")
	}
	env, _, _ := testEnvironment(t)
	m, _ := New(env)
	var out, errOut bytes.Buffer
	app := cli.New(cli.WithIO(strings.NewReader(""), &out, &errOut), cli.WithModule(m), cli.WithEnvironment(func(string) (string, bool) { return "", false }))
	if code := app.Run([]string{"doctor", "--agent"}); code != 0 || !strings.Contains(out.String(), `"checks"`) {
		t.Fatalf("code=%d out=%q err=%q", code, out.String(), errOut.String())
	}
	out.Reset()
	errOut.Reset()
	if code := app.Run([]string{"doctor", "extra"}); code != 2 {
		t.Fatalf("usage code=%d", code)
	}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("render") }

func TestRedaction(t *testing.T) {
	env, creds, _ := testEnvironment(t)
	creds.err = errors.New("keychain provider /Users/private username access-secret 10.0.0.1")
	env.Resolver = resolverFunc(func(context.Context, string) ([]string, error) {
		return nil, errors.New("username token 10.0.0.1 /Users/private")
	})
	env.UserHomeDir = func() (string, error) { return "/Users/private", nil }
	env.Lstat = func(string) (os.FileInfo, error) { return nil, errors.New("filesystem /Users/private refresh-secret") }
	env.ProbeTransport = roundTripFunc(func(*http.Request) (*http.Response, error) { return nil, errors.New("TLS 10.0.0.1 access-secret") })
	m, _ := New(env)
	raw, err := json.Marshal(runReport(t, m, context.Background()))
	if err != nil {
		t.Fatal(err)
	}
	env, _, _ = testEnvironment(t)
	env.ProbeTransport = roundTripFunc(func(*http.Request) (*http.Response, error) {
		return nil, errors.New("TLS 10.0.0.1 access-secret /Users/private")
	})
	m, _ = New(env)
	probeRaw, err := json.Marshal(runReport(t, m, context.Background()))
	if err != nil {
		t.Fatal(err)
	}
	raw = append(raw, probeRaw...)
	for _, secret := range []string{"access-secret", "refresh-secret", "10.0.0.1", "/Users/", "username"} {
		if bytes.Contains(raw, []byte(secret)) {
			t.Fatalf("leaked %q in %s", secret, raw)
		}
	}
}
