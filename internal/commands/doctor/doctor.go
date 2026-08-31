package doctor

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/DaikuFi/daiku-cli/internal/agent"
	"github.com/DaikuFi/daiku-cli/internal/cli"
	"github.com/DaikuFi/daiku-cli/internal/credentials"
	"github.com/DaikuFi/daiku-cli/internal/profiles"
	"github.com/spf13/cobra"
	"golang.org/x/sys/unix"
)

type Status string

const (
	Pass    Status = "pass"
	Warning Status = "warning"
	Fail    Status = "fail"
)

type Check struct {
	Code    string `json:"code"`
	Status  Status `json:"status"`
	Message string `json:"message"`
	Hint    string `json:"hint,omitempty"`
	Details any    `json:"details,omitempty"`
}

type Summary struct {
	Passed   int `json:"passed"`
	Warnings int `json:"warnings"`
	Failed   int `json:"failed"`
}

type Report struct {
	Status  Status  `json:"status"`
	Checks  []Check `json:"checks"`
	Summary Summary `json:"summary"`
}

type Resolver interface {
	LookupHost(context.Context, string) ([]string, error)
}

type CredentialReader interface {
	Get(string) (credentials.Token, error)
}

type Environment struct {
	Version        string
	ProfileStore   profiles.Store
	Credentials    CredentialReader
	Executable     func() (string, error)
	LookPath       func(string) (string, error)
	UserHomeDir    func() (string, error)
	Lstat          func(string) (os.FileInfo, error)
	ReadSkillFile  func(string) ([]byte, error)
	LookupEnv      func(string) (string, bool)
	Now            func() time.Time
	Resolver       Resolver
	ProbeTransport http.RoundTripper
	ProbeTimeout   time.Duration
	Commands       func() []agent.Command
	SkillDigests   map[string]string
	MCPReady       func(context.Context) error
	ListenLoopback func(context.Context) (io.Closer, error)
	SchemaCommit   string
	SchemaSHA256   string
}

type Module struct{ env Environment }

func New(env Environment) (Module, error) {
	if env.Credentials == nil || env.Commands == nil || env.MCPReady == nil || env.ProbeTransport == nil || len(env.SkillDigests) == 0 {
		return Module{}, errors.New("doctor dependencies are incomplete")
	}
	if env.Executable == nil {
		env.Executable = os.Executable
	}
	if env.LookPath == nil {
		env.LookPath = func(name string) (string, error) { return filepath.Abs(name) }
	}
	if env.UserHomeDir == nil {
		env.UserHomeDir = os.UserHomeDir
	}
	if env.Lstat == nil {
		env.Lstat = os.Lstat
	}
	if env.ReadSkillFile == nil {
		env.ReadSkillFile = secureSkillRead
	}
	if env.LookupEnv == nil {
		env.LookupEnv = os.LookupEnv
	}
	if env.Now == nil {
		env.Now = time.Now
	}
	if env.Resolver == nil {
		env.Resolver = net.DefaultResolver
	}
	if env.ProbeTimeout <= 0 {
		env.ProbeTimeout = 5 * time.Second
	}
	if env.ListenLoopback == nil {
		env.ListenLoopback = func(ctx context.Context) (io.Closer, error) {
			return (&net.ListenConfig{}).Listen(ctx, "tcp", "127.0.0.1:0")
		}
	}
	return Module{env: env}, nil
}

func (m Module) Register(root *cobra.Command) {
	root.AddCommand(agent.ReadOnly(&cobra.Command{
		Use: "doctor", Short: "Diagnose the Daiku CLI installation and integrations", Args: cli.UsageArgs(cobra.NoArgs),
		RunE: func(command *cobra.Command, _ []string) error {
			report, err := m.run(command.Context())
			if err != nil {
				return err
			}
			machine, _ := command.Flags().GetBool("json")
			if machine {
				return cli.WriteSuccess(command.OutOrStdout(), report)
			}
			return renderHuman(command.OutOrStdout(), report, language(command))
		},
	}))
}

func language(command *cobra.Command) string {
	v, _ := command.Flags().GetString("language")
	if v == "es" {
		return "es"
	}
	return "en"
}

func (m Module) run(ctx context.Context) (Report, error) {
	checks := make([]Check, 0, 12)
	add := func(c Check) { checks = append(checks, c) }
	stop := func() error {
		if ctx.Err() != nil {
			return &cli.Error{Code: "operation_cancelled", Message: "operation cancelled", ExitCode: cli.ExitConflict}
		}
		return nil
	}
	if err := stop(); err != nil {
		return Report{}, err
	}
	installation, err := m.installation(ctx)
	if err != nil {
		return Report{}, cancellationError()
	}
	add(installation)
	if err := stop(); err != nil {
		return Report{}, err
	}
	add(versionCheck(m.env.Version))
	if err := stop(); err != nil {
		return Report{}, err
	}
	cfg, profileName, profile, profileOK := m.profile()
	if err := stop(); err != nil {
		return Report{}, err
	}
	add(profile)
	if err := stop(); err != nil {
		return Report{}, err
	}
	token, tokenOK, credential := m.credential(profileName, profileOK)
	if err := stop(); err != nil {
		return Report{}, err
	}
	add(credential)
	if err := stop(); err != nil {
		return Report{}, err
	}
	auth, mayProbe := m.authentication(token, tokenOK)
	add(auth)
	if err := stop(); err != nil {
		return Report{}, err
	}
	host, dnsOK, dns := m.dns(profileName, cfg, profileOK, ctx)
	if err := stop(); err != nil {
		return Report{}, err
	}
	add(dns)
	if err := stop(); err != nil {
		return Report{}, err
	}
	add(m.probe(ctx, host, cfg, token, mayProbe && dnsOK))
	if err := stop(); err != nil {
		return Report{}, err
	}
	add(m.schema())
	if err := stop(); err != nil {
		return Report{}, err
	}
	codex, err := m.skill(ctx, "codex", "CODEX_HOME")
	if err != nil {
		return Report{}, cancellationError()
	}
	add(codex)
	if err := stop(); err != nil {
		return Report{}, err
	}
	claude, err := m.skill(ctx, "claude", "CLAUDE_HOME")
	if err != nil {
		return Report{}, cancellationError()
	}
	add(claude)
	if err := stop(); err != nil {
		return Report{}, err
	}
	add(m.mcp(ctx))
	if err := stop(); err != nil {
		return Report{}, err
	}
	callback, err := m.callback(ctx)
	if err != nil {
		return Report{}, cancellationError()
	}
	add(callback)
	if err := stop(); err != nil {
		return Report{}, err
	}
	return aggregate(checks), nil
}

func cancellationError() error {
	return &cli.Error{Code: "operation_cancelled", Message: "operation cancelled", ExitCode: cli.ExitConflict}
}

func (m Module) schema() Check {
	details := struct {
		Commit string `json:"commit"`
		SHA256 string `json:"sha256"`
	}{Commit: m.env.SchemaCommit, SHA256: m.env.SchemaSHA256}
	if details.Commit == "" || details.SHA256 == "" {
		return Check{Code: "schema_metadata_invalid", Status: Fail, Message: "compiled local API schema metadata is incomplete"}
	}
	return Check{Code: "schema_compatibility_unknown", Status: Warning, Message: "local API schema metadata is compiled in, but the server provided no compatibility signal", Hint: "compatibility cannot be inferred from reachability alone", Details: details}
}

func (m Module) installation(ctx context.Context) (Check, error) {
	exe, err := m.env.Executable()
	if ctx.Err() != nil {
		return Check{}, ctx.Err()
	}
	path, pathErr := m.env.LookPath("daiku")
	if ctx.Err() != nil {
		return Check{}, ctx.Err()
	}
	if err != nil || pathErr != nil {
		return Check{Code: "installation_path_missing", Status: Warning, Message: "the running executable is not discoverable as daiku on PATH", Hint: "install daiku or add its directory to PATH"}, nil
	}
	if filepath.Clean(exe) != filepath.Clean(path) {
		return Check{Code: "installation_path_mismatch", Status: Warning, Message: "PATH resolves daiku to a different executable", Hint: "remove stale installations or reorder PATH"}, nil
	}
	return Check{Code: "installation_path_ok", Status: Pass, Message: "the running executable matches PATH"}, nil
}

var semver = regexp.MustCompile(`^v?([0-9]+)\.([0-9]+)\.([0-9]+)(?:[-+][0-9A-Za-z.-]+)?$`)

func versionCheck(v string) Check {
	if v == "dev" || strings.TrimSpace(v) == "" {
		return Check{Code: "version_development", Status: Warning, Message: "this is a development build", Hint: "use a tagged release for production workflows"}
	}
	if !semver.MatchString(v) {
		return Check{Code: "version_malformed", Status: Fail, Message: "the CLI version is malformed", Hint: "reinstall a signed Daiku release"}
	}
	return Check{Code: "version_ok", Status: Pass, Message: "the CLI version is well formed"}
}

func (m Module) profile() (profiles.Config, string, Check, bool) {
	cfg, err := m.env.ProfileStore.Load()
	if err != nil {
		return cfg, "", Check{Code: "profile_invalid", Status: Fail, Message: "the profile configuration is corrupt or has unsafe permissions", Hint: "repair the Daiku profile configuration"}, false
	}
	if cfg.Current == "" {
		return cfg, "", Check{Code: "profile_missing", Status: Warning, Message: "no profile is selected", Hint: "create or select a profile"}, false
	}
	p := cfg.Profiles[cfg.Current]
	if _, err := profiles.NormalizeAPIURL(p.APIURL); err != nil {
		return cfg, "", Check{Code: "profile_invalid", Status: Fail, Message: "the selected profile is invalid", Hint: "repair the selected profile"}, false
	}
	return cfg, cfg.Current, Check{Code: "profile_ok", Status: Pass, Message: "the selected profile is valid and owner-private"}, true
}

func (m Module) credential(profile string, ok bool) (credentials.Token, bool, Check) {
	if !ok {
		return credentials.Token{}, false, Check{Code: "credentials_not_checked", Status: Warning, Message: "credentials could not be checked without a valid selected profile"}
	}
	token, err := m.env.Credentials.Get(profile)
	if errors.Is(err, credentials.ErrNotFound) {
		return token, false, Check{Code: "credentials_missing", Status: Warning, Message: "no credentials are stored for the selected profile", Hint: "run daiku auth login"}
	}
	if err != nil {
		return token, false, Check{Code: "credential_store_unavailable", Status: Fail, Message: "the credential store is unavailable or contains unsafe data", Hint: "check the configured credential provider"}
	}
	if token.AccessToken == "" || token.RefreshToken == "" || token.ExpiresAt <= 0 {
		return token, false, Check{Code: "credentials_incomplete", Status: Fail, Message: "stored credentials are incomplete", Hint: "sign in again"}
	}
	return token, true, Check{Code: "credentials_ok", Status: Pass, Message: "the credential store is available and credentials have the expected shape"}
}

func (m Module) authentication(token credentials.Token, ok bool) (Check, bool) {
	if !ok {
		return Check{Code: "authentication_unavailable", Status: Warning, Message: "authentication cannot be validated without complete credentials"}, false
	}
	if !m.env.Now().Before(time.Unix(token.ExpiresAt, 0)) {
		return Check{Code: "token_expired", Status: Fail, Message: "the access token is expired and was not sent", Hint: "run daiku auth login"}, false
	}
	scopes := strings.Fields(token.Scope)
	hasRead := false
	for _, scope := range scopes {
		if scope == "finance:read" {
			hasRead = true
		}
	}
	if !hasRead {
		return Check{Code: "token_scope_incomplete", Status: Fail, Message: "the access token lacks the required read scope", Hint: "sign in again with finance:read access"}, false
	}
	return Check{Code: "authentication_ok", Status: Pass, Message: "the access token is unexpired and includes the required read scope"}, true
}

func selectedURL(cfg profiles.Config, name string) (string, bool) {
	if name == "" {
		return "", false
	}
	v, err := profiles.NormalizeAPIURL(cfg.Profiles[name].APIURL)
	return v, err == nil
}

func (m Module) dns(profile string, cfg profiles.Config, ok bool, ctx context.Context) (string, bool, Check) {
	base, valid := selectedURL(cfg, profile)
	if !ok || !valid {
		return "", false, Check{Code: "dns_not_checked", Status: Warning, Message: "DNS was not checked without a valid API hostname"}
	}
	u, _ := url.Parse(base)
	host := u.Hostname()
	if host == "localhost" || host == "127.0.0.1" {
		return host, true, Check{Code: "dns_local", Status: Pass, Message: "the validated API hostname is local"}
	}
	if _, err := m.env.Resolver.LookupHost(ctx, host); err != nil {
		return host, false, Check{Code: "dns_failed", Status: Fail, Message: "the validated API hostname did not resolve", Hint: "check DNS and network connectivity"}
	}
	return host, true, Check{Code: "dns_ok", Status: Pass, Message: "the validated API hostname resolves"}
}

func (m Module) probe(ctx context.Context, _ string, cfg profiles.Config, token credentials.Token, safe bool) Check {
	if !safe {
		return Check{Code: "api_probe_skipped", Status: Warning, Message: "the API probe was skipped because sending credentials was not safe"}
	}
	base, _ := selectedURL(cfg, cfg.Current)
	probeCtx, cancel := context.WithTimeout(ctx, m.env.ProbeTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(probeCtx, http.MethodGet, base, nil)
	if err != nil {
		return Check{Code: "api_probe_failed", Status: Fail, Message: "the API probe could not be constructed"}
	}
	req.Header.Set("Authorization", "Bearer "+token.AccessToken)
	// RoundTrip is invoked directly exactly once. A dedicated transport is
	// injected by the composition root, so net/http client redirect and retry
	// policy can never replay this bearer request.
	resp, err := m.env.ProbeTransport.RoundTrip(req)
	if err != nil {
		if resp != nil && resp.Body != nil {
			_ = resp.Body.Close()
		}
		if errors.Is(probeCtx.Err(), context.DeadlineExceeded) {
			return Check{Code: "api_timeout", Status: Fail, Message: "the bounded API probe timed out"}
		}
		return Check{Code: "api_unreachable", Status: Fail, Message: "the API probe failed before receiving a response", Hint: "check TLS and network connectivity"}
	}
	if resp == nil {
		return Check{Code: "api_unreachable", Status: Fail, Message: "the API probe failed before receiving a response", Hint: "check TLS and network connectivity"}
	}
	if resp.Body == nil {
		resp.Body = http.NoBody
	}
	defer resp.Body.Close()
	switch {
	case resp.StatusCode >= 200 && resp.StatusCode < 400:
		return Check{Code: "api_ok", Status: Pass, Message: "the API accepted the bounded authentication probe"}
	case resp.StatusCode == 401:
		return Check{Code: "api_unauthorized", Status: Fail, Message: "the API rejected the credentials", Hint: "run daiku auth login"}
	case resp.StatusCode == 403:
		return Check{Code: "api_forbidden", Status: Fail, Message: "the API denied the authenticated probe", Hint: "check the selected profile permissions"}
	case resp.StatusCode == 429:
		return Check{Code: "api_rate_limited", Status: Warning, Message: "the API rate limited the single probe", Hint: "wait before running diagnostics again"}
	case resp.StatusCode >= 500:
		return Check{Code: "api_server_error", Status: Fail, Message: "the API was reachable but unavailable", Hint: "retry after the service recovers"}
	default:
		return Check{Code: "api_unexpected_status", Status: Warning, Message: "the API returned an unexpected status to the probe", Hint: "check the configured endpoint"}
	}
}

const maxSkillFileSize = int64(2 << 20)

type integrityManifest struct {
	Version int               `json:"version"`
	Files   map[string]string `json:"files"`
}

func (m Module) skill(ctx context.Context, name, envName string) (Check, error) {
	home, override := m.env.LookupEnv(envName)
	if !override {
		var err error
		home, err = m.env.UserHomeDir()
		if ctx.Err() != nil {
			return Check{}, ctx.Err()
		}
		if err != nil || !filepath.IsAbs(home) {
			return skillFinding(name, "home_unavailable", Fail, "the agent home directory is unavailable", "set an absolute agent home directory"), nil
		}
		home = filepath.Join(home, "."+name)
	} else if home == "" || !filepath.IsAbs(home) {
		return skillFinding(name, "home_invalid", Fail, "the configured agent home directory is invalid", "use an absolute agent home directory"), nil
	}
	dir := filepath.Join(home, "skills", "daiku")
	if err := m.checkSkillDirectory(ctx, home); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return skillFinding(name, "skill_absent", Warning, "Daiku skill is not installed", "run the official Daiku skill installer"), nil
		}
		if ctx.Err() != nil {
			return Check{}, ctx.Err()
		}
		return skillFinding(name, "skill_unsafe", Fail, "Daiku skill directories are unsafe", "reinstall the skill into owner-controlled directories"), nil
	}
	for _, path := range []string{filepath.Join(home, "skills"), dir, filepath.Join(dir, "agents"), filepath.Join(dir, "references")} {
		if err := m.checkSkillDirectory(ctx, path); err != nil {
			if ctx.Err() != nil {
				return Check{}, ctx.Err()
			}
			if errors.Is(err, os.ErrNotExist) {
				return skillFinding(name, "skill_absent", Warning, "Daiku skill is incomplete", "run the official Daiku skill installer"), nil
			}
			return skillFinding(name, "skill_unsafe", Fail, "Daiku skill directories are unsafe", "reinstall the skill into owner-controlled directories"), nil
		}
	}
	integrityRaw, err := m.readSkill(ctx, filepath.Join(dir, "integrity.json"))
	if err != nil {
		if ctx.Err() != nil {
			return Check{}, ctx.Err()
		}
		return skillFinding(name, "skill_corrupt", Fail, "Daiku skill integrity metadata is missing or unsafe", "reinstall the skill"), nil
	}
	var manifest integrityManifest
	if json.Unmarshal(integrityRaw, &manifest) != nil || manifest.Version != 1 || !sameDigests(manifest.Files, m.env.SkillDigests) {
		return skillFinding(name, "skill_corrupt", Fail, "Daiku skill integrity metadata is malformed or untrusted", "reinstall the skill"), nil
	}
	var commandsRaw []byte
	paths := make([]string, 0, len(m.env.SkillDigests))
	for path := range m.env.SkillDigests {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	for _, path := range paths {
		raw, err := m.readSkill(ctx, filepath.Join(dir, filepath.FromSlash(path)))
		if err != nil {
			if ctx.Err() != nil {
				return Check{}, ctx.Err()
			}
			return skillFinding(name, "skill_corrupt", Fail, "Daiku skill contains a missing, unsafe, special, or oversized file", "reinstall the skill"), nil
		}
		digest := fmt.Sprintf("%x", sha256.Sum256(raw))
		if digest != manifest.Files[path] {
			status := Fail
			suffix := "skill_corrupt"
			message := "Daiku skill content failed integrity validation"
			if strings.HasPrefix(path, "references/") {
				status = Warning
				suffix = "skill_stale"
				message = "Daiku skill references are stale"
			}
			return skillFinding(name, suffix, status, message, "reinstall the skill"), nil
		}
		if path == "references/commands.json" {
			commandsRaw = raw
		}
	}
	var commandManifest struct {
		Commands []agent.Command `json:"commands"`
	}
	if json.Unmarshal(commandsRaw, &commandManifest) != nil {
		return skillFinding(name, "skill_corrupt", Fail, "Daiku skill command metadata is malformed", "reinstall the skill"), nil
	}
	if ctx.Err() != nil {
		return Check{}, ctx.Err()
	}
	live := m.env.Commands()
	if ctx.Err() != nil {
		return Check{}, ctx.Err()
	}
	want, _ := json.Marshal(canonicalCommands(live))
	got, _ := json.Marshal(canonicalCommands(commandManifest.Commands))
	if !bytes.Equal(want, got) {
		return skillFinding(name, "skill_stale", Warning, "Daiku skill is stale relative to live command metadata", "reinstall the skill"), nil
	}
	return skillFinding(name, "skill_ok", Pass, "Daiku skill files and live command metadata passed integrity checks", ""), nil
}

func skillFinding(name, suffix string, status Status, message, hint string) Check {
	return Check{Code: name + "_" + suffix, Status: status, Message: name + " " + message, Hint: hint}
}

func sameDigests(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for path, digest := range b {
		if a[path] != digest {
			return false
		}
	}
	return true
}

func (m Module) checkSkillDirectory(ctx context.Context, path string) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	info, err := m.env.Lstat(path)
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if err != nil {
		return err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm()&0o022 != 0 || !ownedByCurrentUser(info) {
		return errors.New("unsafe skill directory")
	}
	return nil
}

func (m Module) readSkill(ctx context.Context, path string) ([]byte, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	raw, err := m.env.ReadSkillFile(path)
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	return raw, err
}

func ownedByCurrentUser(info os.FileInfo) bool {
	stat, ok := info.Sys().(*syscall.Stat_t)
	return ok && stat.Uid == uint32(os.Geteuid())
}

func secureSkillRead(path string) ([]byte, error) {
	fd, err := unix.Open(path, unix.O_RDONLY|unix.O_NONBLOCK|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, err
	}
	file := os.NewFile(uintptr(fd), path)
	defer file.Close()
	var stat unix.Stat_t
	if err := unix.Fstat(fd, &stat); err != nil {
		return nil, err
	}
	if stat.Mode&unix.S_IFMT != unix.S_IFREG || stat.Uid != uint32(os.Geteuid()) || stat.Mode&0o022 != 0 || stat.Size > maxSkillFileSize {
		return nil, errors.New("unsafe skill file")
	}
	raw, err := io.ReadAll(io.LimitReader(file, maxSkillFileSize+1))
	if err != nil || int64(len(raw)) > maxSkillFileSize {
		return nil, errors.New("invalid skill file")
	}
	return raw, nil
}

func canonicalCommands(v []agent.Command) []agent.Command {
	out := append([]agent.Command(nil), v...)
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out
}

func (m Module) mcp(ctx context.Context) Check {
	probeCtx, cancel := context.WithTimeout(ctx, m.env.ProbeTimeout)
	defer cancel()
	if err := m.env.MCPReady(probeCtx); err != nil {
		if probeCtx.Err() != nil {
			return Check{Code: "mcp_cancelled", Status: Fail, Message: "the in-memory MCP readiness check was cancelled"}
		}
		return Check{Code: "mcp_failed", Status: Fail, Message: "the in-memory MCP server failed initialize or list-tools readiness", Hint: "check the MCP installation"}
	}
	return Check{Code: "mcp_ok", Status: Pass, Message: "the in-memory MCP server initialized and listed tools"}
}

func (m Module) callback(ctx context.Context) (Check, error) {
	if ctx.Err() != nil {
		return Check{}, ctx.Err()
	}
	listener, err := m.env.ListenLoopback(ctx)
	if ctx.Err() != nil {
		if listener != nil {
			_ = listener.Close()
		}
		return Check{}, ctx.Err()
	}
	if err != nil {
		return Check{Code: "oauth_callback_blocked", Status: Fail, Message: "an ephemeral loopback OAuth callback cannot be opened", Hint: "allow local loopback listeners"}, nil
	}
	if err := listener.Close(); err != nil {
		return Check{Code: "oauth_callback_close_failed", Status: Fail, Message: "the ephemeral OAuth callback could not be closed cleanly", Hint: "check local network restrictions"}, nil
	}
	return Check{Code: "oauth_callback_ok", Status: Pass, Message: "an ephemeral loopback OAuth callback is available"}, nil
}

func aggregate(checks []Check) Report {
	r := Report{Status: Pass, Checks: checks}
	for _, c := range checks {
		switch c.Status {
		case Pass:
			r.Summary.Passed++
		case Warning:
			r.Summary.Warnings++
			if r.Status == Pass {
				r.Status = Warning
			}
		case Fail:
			r.Summary.Failed++
			r.Status = Fail
		}
	}
	return r
}

func renderHuman(w io.Writer, report Report, lang string) error {
	labels := map[Status]string{Pass: "PASS", Warning: "WARN", Fail: "FAIL"}
	summary := "%d passed, %d warnings, %d failed"
	hintLabel := "Hint"
	if lang == "es" {
		labels = map[Status]string{Pass: "BIEN", Warning: "AVISO", Fail: "FALLO"}
		summary = "%d correctas, %d avisos, %d fallidas"
		hintLabel = "Sugerencia"
	}
	for _, c := range report.Checks {
		message := c.Message
		hint := c.Hint
		if lang == "es" {
			message, hint = spanishText(c.Code, message, hint)
		}
		if _, err := fmt.Fprintf(w, "[%s] %s: %s\n", labels[c.Status], c.Code, message); err != nil {
			return err
		}
		if hint != "" {
			if _, err := fmt.Fprintf(w, "  %s: %s\n", hintLabel, hint); err != nil {
				return err
			}
		}
	}
	_, err := fmt.Fprintf(w, summary+"\n", report.Summary.Passed, report.Summary.Warnings, report.Summary.Failed)
	return err
}

type localizedCheck struct{ message, hint string }

func spanishText(code, fallbackMessage, fallbackHint string) (string, string) {
	texts := map[string]localizedCheck{
		"installation_path_ok":         {"el ejecutable en uso coincide con PATH", ""},
		"installation_path_missing":    {"el ejecutable en uso no se encuentra como daiku en PATH", "instala daiku o añade su directorio a PATH"},
		"installation_path_mismatch":   {"PATH resuelve daiku a otro ejecutable", "elimina instalaciones antiguas o reordena PATH"},
		"version_ok":                   {"la versión del CLI tiene un formato válido", ""},
		"version_development":          {"esta es una compilación de desarrollo", "usa una versión etiquetada para flujos de producción"},
		"version_malformed":            {"la versión del CLI tiene un formato inválido", "reinstala una versión firmada de Daiku"},
		"profile_ok":                   {"el perfil seleccionado es válido y privado para su propietario", ""},
		"profile_missing":              {"no hay un perfil seleccionado", "crea o selecciona un perfil"},
		"profile_invalid":              {"la configuración del perfil está dañada, es inválida o tiene permisos inseguros", "repara la configuración del perfil de Daiku"},
		"credentials_not_checked":      {"no se pudieron comprobar las credenciales sin un perfil válido", ""},
		"credentials_missing":          {"no hay credenciales guardadas para el perfil seleccionado", "ejecuta daiku auth login"},
		"credential_store_unavailable": {"el almacén de credenciales no está disponible o contiene datos inseguros", "comprueba el proveedor de credenciales configurado"},
		"credentials_incomplete":       {"las credenciales guardadas están incompletas", "vuelve a iniciar sesión"},
		"credentials_ok":               {"el almacén está disponible y las credenciales tienen la forma esperada", ""},
		"authentication_unavailable":   {"no se puede validar la autenticación sin credenciales completas", ""},
		"token_expired":                {"el token de acceso venció y no fue enviado", "ejecuta daiku auth login"},
		"token_scope_incomplete":       {"al token le falta el permiso de lectura requerido", "vuelve a iniciar sesión con acceso finance:read"},
		"authentication_ok":            {"el token no ha vencido e incluye el permiso de lectura requerido", ""},
		"dns_not_checked":              {"no se comprobó DNS porque no había un host de API válido", ""},
		"dns_local":                    {"el host validado de la API es local", ""},
		"dns_failed":                   {"el host validado de la API no resolvió", "comprueba DNS y la conectividad de red"},
		"dns_ok":                       {"el host validado de la API resuelve", ""},
		"api_probe_skipped":            {"se omitió la comprobación de API porque no era seguro enviar credenciales", ""},
		"api_probe_failed":             {"no se pudo construir la comprobación de API", ""},
		"api_timeout":                  {"la única comprobación acotada de API agotó el tiempo", ""},
		"api_unreachable":              {"la comprobación de API falló antes de recibir una respuesta", "comprueba TLS y la conectividad de red"},
		"api_ok":                       {"la API aceptó la comprobación acotada de autenticación", ""},
		"api_unauthorized":             {"la API rechazó las credenciales", "ejecuta daiku auth login"},
		"api_forbidden":                {"la API denegó la comprobación autenticada", "comprueba los permisos del perfil seleccionado"},
		"api_rate_limited":             {"la API limitó la única comprobación", "espera antes de volver a ejecutar el diagnóstico"},
		"api_server_error":             {"la API respondió pero no estaba disponible", "vuelve a intentarlo cuando el servicio se recupere"},
		"api_unexpected_status":        {"la API devolvió un estado inesperado", "comprueba la configuración del endpoint"},
		"schema_metadata_invalid":      {"los metadatos compilados del esquema local están incompletos", "reinstala una versión firmada de Daiku"},
		"schema_compatibility_unknown": {"los metadatos del esquema local están compilados, pero el servidor no informó compatibilidad", "la compatibilidad no se puede inferir sólo de la conectividad"},
		"mcp_ok":                       {"el servidor MCP en memoria inició y enumeró sus herramientas", ""},
		"mcp_failed":                   {"el servidor MCP en memoria falló al iniciar o enumerar herramientas", "comprueba la instalación MCP"},
		"mcp_cancelled":                {"se canceló la comprobación MCP en memoria", ""},
		"oauth_callback_ok":            {"hay disponible un callback OAuth efímero en loopback", ""},
		"oauth_callback_blocked":       {"no se puede abrir un callback OAuth efímero en loopback", "permite listeners locales en loopback"},
		"oauth_callback_close_failed":  {"el callback OAuth efímero no se pudo cerrar correctamente", "comprueba las restricciones locales de red"},
	}
	for _, agentName := range []string{"codex", "claude"} {
		label := "Codex"
		if agentName == "claude" {
			label = "Claude"
		}
		texts[agentName+"_home_unavailable"] = localizedCheck{"el directorio de " + label + " no está disponible", "configura un directorio absoluto para el agente"}
		texts[agentName+"_home_invalid"] = localizedCheck{"el directorio configurado de " + label + " es inválido", "usa un directorio absoluto para el agente"}
		texts[agentName+"_skill_absent"] = localizedCheck{"la skill Daiku de " + label + " no está instalada o está incompleta", "ejecuta el instalador oficial de la skill"}
		texts[agentName+"_skill_unsafe"] = localizedCheck{"los directorios de la skill Daiku de " + label + " son inseguros", "reinstala la skill en directorios controlados por su propietario"}
		texts[agentName+"_skill_corrupt"] = localizedCheck{"la skill Daiku de " + label + " no superó la verificación de integridad", "reinstala la skill"}
		texts[agentName+"_skill_stale"] = localizedCheck{"la skill Daiku de " + label + " está desactualizada", "reinstala la skill"}
		texts[agentName+"_skill_ok"] = localizedCheck{"la skill Daiku de " + label + " y sus comandos superaron la verificación", ""}
	}
	if text, ok := texts[code]; ok {
		return text.message, text.hint
	}
	return fallbackMessage, fallbackHint
}
