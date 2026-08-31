package main

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/DaikuFi/daiku-cli/internal/agent"
	authcore "github.com/DaikuFi/daiku-cli/internal/auth"
	"github.com/DaikuFi/daiku-cli/internal/cli"
	authcommand "github.com/DaikuFi/daiku-cli/internal/commands/auth"
	budgetcommand "github.com/DaikuFi/daiku-cli/internal/commands/budgets"
	catalogcommand "github.com/DaikuFi/daiku-cli/internal/commands/catalog"
	mcpcommand "github.com/DaikuFi/daiku-cli/internal/commands/mcp"
	portfoliocommand "github.com/DaikuFi/daiku-cli/internal/commands/portfolios"
	profilecommand "github.com/DaikuFi/daiku-cli/internal/commands/profile"
	projectioncommand "github.com/DaikuFi/daiku-cli/internal/commands/projections"
	recurringcommand "github.com/DaikuFi/daiku-cli/internal/commands/recurring"
	transactioncommand "github.com/DaikuFi/daiku-cli/internal/commands/transactions"
	versioncommand "github.com/DaikuFi/daiku-cli/internal/commands/version"
	"github.com/DaikuFi/daiku-cli/internal/credentials"
	"github.com/DaikuFi/daiku-cli/internal/mcpserver"
	"github.com/DaikuFi/daiku-cli/internal/profiles"
)

var version = "dev"

func main() {
	configPath, err := profiles.DefaultPath()
	if err != nil {
		configPath = filepath.Join(".", ".daiku", "config.json")
	}
	profileStore := profiles.Store{Path: configPath}
	credentialStore := credentials.Store(credentials.Keyring{})
	// The file fallback is explicit because a keychain failure must never silently
	// weaken credential storage.
	if os.Getenv("DAIKU_CREDENTIAL_STORE") == "file" {
		credentialStore = credentials.FileStore{Dir: filepath.Join(filepath.Dir(configPath), "credentials")}
	}
	oauthClient, err := authcore.New(authcore.Config{ClientID: "daiku-cli", AuthorizeURL: "https://api.daiku.app/oauth/authorize/", TokenURL: "https://api.daiku.app/oauth/token/", RevokeURL: "https://api.daiku.app/oauth/revoke_token/", Scopes: []string{"finance:read", "finance:write"}})
	if err != nil {
		panic("invalid built-in OAuth configuration")
	}
	authManager := &authcore.Manager{Store: credentialStore, OAuth: oauthClient}
	var newApp func(context.Context, io.Reader, io.Writer, io.Writer) *cli.App
	executor := &commandExecutor{
		newApp: func(ctx context.Context, in io.Reader, out, errOut io.Writer) *cli.App {
			return newApp(ctx, in, out, errOut)
		},
		gate: make(chan struct{}, 1),
	}
	runMCP := func(ctx context.Context, allowWrites bool, in io.ReadCloser, out io.WriteCloser, errOut io.Writer) error {
		logger := slog.New(slog.NewTextHandler(errOut, nil))
		return mcpserver.Run(ctx, executor, mcpserver.Options{AllowWrites: allowWrites, Version: version, Logger: logger}, in, out)
	}
	newApp = func(ctx context.Context, in io.Reader, out, errOut io.Writer) *cli.App {
		return cli.New(
			cli.WithContext(ctx),
			cli.WithIO(in, out, errOut),
			cli.WithVersion(version),
			cli.WithModule(versioncommand.New(version)),
			cli.WithModule(profilecommand.New(profileStore, credentialStore)),
			cli.WithModule(authcommand.New(profileStore, credentialStore, oauthClient)),
			cli.WithModule(catalogcommand.New(profileStore, authManager, nil)),
			cli.WithModule(transactioncommand.New(transactioncommand.GeneratedServiceFactory(profileStore, authManager))),
			cli.WithModule(budgetcommand.New(profileStore, authManager)),
			cli.WithModule(recurringcommand.New(profileStore, authManager)),
			cli.WithModule(portfoliocommand.New(portfoliocommand.GeneratedFactory(profileStore, authManager, nil))),
			cli.WithModule(projectioncommand.New(profileStore, authManager)),
			cli.WithModule(mcpcommand.New(runMCP)),
		)
	}
	app := newApp(context.Background(), os.Stdin, os.Stdout, os.Stderr)
	os.Exit(app.Run(os.Args[1:]))
}

type commandExecutor struct {
	newApp func(context.Context, io.Reader, io.Writer, io.Writer) *cli.App
	gate   chan struct{}
}

func (e *commandExecutor) Commands() []agent.Command {
	return e.newApp(context.Background(), strings.NewReader(""), io.Discard, io.Discard).Commands()
}

func (e *commandExecutor) Execute(ctx context.Context, args []string) mcpserver.Execution {
	select {
	case e.gate <- struct{}{}:
		defer func() { <-e.gate }()
	case <-ctx.Done():
		return mcpserver.Execution{}
	}
	if ctx.Err() != nil {
		return mcpserver.Execution{}
	}

	var stdout, stderr bytes.Buffer
	app := e.newApp(ctx, strings.NewReader(""), &stdout, &stderr)
	return mcpserver.Execution{ExitCode: app.Run(args), Stdout: stdout.Bytes(), Stderr: stderr.Bytes()}
}
