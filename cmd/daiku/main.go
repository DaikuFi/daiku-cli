package main

import (
	"os"
	"path/filepath"

	authcore "github.com/DaikuFi/daiku-cli/internal/auth"
	"github.com/DaikuFi/daiku-cli/internal/cli"
	authcommand "github.com/DaikuFi/daiku-cli/internal/commands/auth"
	budgetcommand "github.com/DaikuFi/daiku-cli/internal/commands/budgets"
	catalogcommand "github.com/DaikuFi/daiku-cli/internal/commands/catalog"
	portfoliocommand "github.com/DaikuFi/daiku-cli/internal/commands/portfolios"
	profilecommand "github.com/DaikuFi/daiku-cli/internal/commands/profile"
	projectioncommand "github.com/DaikuFi/daiku-cli/internal/commands/projections"
	recurringcommand "github.com/DaikuFi/daiku-cli/internal/commands/recurring"
	transactioncommand "github.com/DaikuFi/daiku-cli/internal/commands/transactions"
	versioncommand "github.com/DaikuFi/daiku-cli/internal/commands/version"
	"github.com/DaikuFi/daiku-cli/internal/credentials"
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
	app := cli.New(
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
	)
	os.Exit(app.Run(os.Args[1:]))
}
