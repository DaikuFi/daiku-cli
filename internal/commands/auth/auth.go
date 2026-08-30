package auth

import (
	"errors"
	"fmt"
	"time"

	authcore "github.com/DaikuFi/daiku-cli/internal/auth"
	"github.com/DaikuFi/daiku-cli/internal/cli"
	"github.com/DaikuFi/daiku-cli/internal/credentials"
	"github.com/DaikuFi/daiku-cli/internal/profiles"
	"github.com/DaikuFi/daiku-cli/internal/prompt"
	"github.com/spf13/cobra"
)

type Module struct {
	Profiles    profiles.Store
	Credentials credentials.Store
	OAuth       *authcore.Client
}

func New(p profiles.Store, c credentials.Store, o *authcore.Client) Module { return Module{p, c, o} }
func (m Module) Register(root *cobra.Command) {
	cmd := &cobra.Command{Use: "auth", Short: "Authenticate with Daiku", Args: cli.UsageArgs(cobra.NoArgs)}
	cmd.AddCommand(m.login(), m.logout(), m.status())
	root.AddCommand(cmd)
}
func (m Module) selected() (string, error) {
	cfg, err := m.Profiles.Load()
	if err != nil {
		return "", safe("config_error", "profile configuration could not be read", cli.ExitFailure)
	}
	if cfg.Current == "" {
		return "", safe("profile_required", "select a profile before authenticating", cli.ExitUsage)
	}
	return cfg.Current, nil
}
func (m Module) login() *cobra.Command {
	return &cobra.Command{Use: "login", Short: "Sign in using OAuth", Args: cli.UsageArgs(cobra.NoArgs), RunE: func(cmd *cobra.Command, _ []string) error {
		profile, err := m.selected()
		if err != nil {
			return err
		}
		jsonOut, _ := cmd.Flags().GetBool("json")
		var manual func(string) error
		if !jsonOut {
			manual = func(target string) error {
				_, notifyErr := fmt.Fprint(cmd.ErrOrStderr(), cli.Human(cmd).Localizer.Humanf("Open this URL to continue authentication:\n%s\n", target))
				return notifyErr
			}
		}
		result, err := m.OAuth.LoginWithManualFallback(cmd.Context(), manual)
		if err != nil {
			return safe("oauth_login_failed", err.Error(), cli.ExitAuth)
		}
		if err = m.Credentials.Put(profile, result.Token); err != nil {
			return safe("credential_store_error", "credentials could not be stored securely", cli.ExitFailure)
		}
		data := map[string]any{"profile": profile, "logged_in": true}
		if jsonOut {
			return cli.WriteSuccess(cmd.OutOrStdout(), data)
		}
		_, err = fmt.Fprint(cmd.OutOrStdout(), cli.Human(cmd).Localizer.Humanf("Logged in as profile %s.\n", profile))
		return err
	}}
}
func (m Module) logout() *cobra.Command {
	var local bool
	var yes bool
	cmd := &cobra.Command{Use: "logout", Short: "Revoke and remove credentials", Args: cli.UsageArgs(cobra.NoArgs), RunE: func(cmd *cobra.Command, _ []string) error {
		profile, err := m.selected()
		if err != nil {
			return err
		}
		if !yes {
			human := cli.Human(cmd)
			confirmation := prompt.Prompter{In: cmd.InOrStdin(), Out: cmd.ErrOrStderr(), Localize: human.Localizer, Terminal: human.Interactive && !human.JSON}
			if err = confirmation.ConfirmDestructive(human.Localizer.Humanf("Log out profile %s.", profile)); err != nil {
				return confirmationError(err)
			}
		}
		if !local {
			manager := authcore.Manager{Store: m.Credentials, OAuth: m.OAuth}
			if err = manager.Logout(cmd.Context(), profile); err != nil {
				return safe("oauth_revoke_failed", "credentials could not be revoked; use --local-only to remove only the local copy", cli.ExitUnavailable)
			}
		} else if err = m.Credentials.Delete(profile); err != nil {
			return safe("credential_store_error", "local credentials could not be removed", cli.ExitFailure)
		}
		jsonOut, _ := cmd.Flags().GetBool("json")
		if jsonOut {
			return cli.WriteSuccess(cmd.OutOrStdout(), map[string]any{"profile": profile, "logged_in": false, "revoked": !local})
		}
		_, err = fmt.Fprint(cmd.OutOrStdout(), cli.Human(cmd).Localizer.Humanf("Logged out profile %s.\n", profile))
		return err
	}}
	cmd.Flags().BoolVar(&local, "local-only", false, "remove local credentials without revoking the token")
	cmd.Flags().BoolVar(&yes, "yes", false, "skip the interactive confirmation")
	return cmd
}
func (m Module) status() *cobra.Command {
	return &cobra.Command{Use: "status", Short: "Show authentication status", Args: cli.UsageArgs(cobra.NoArgs), RunE: func(cmd *cobra.Command, _ []string) error {
		profile, err := m.selected()
		if err != nil {
			return err
		}
		token, err := m.Credentials.Get(profile)
		logged := err == nil
		if err != nil && !errors.Is(err, credentials.ErrNotFound) {
			return safe("credential_store_error", "credentials could not be read securely", cli.ExitFailure)
		}
		data := map[string]any{"profile": profile, "logged_in": logged}
		if logged {
			data["expires_at"] = time.Unix(token.ExpiresAt, 0).UTC().Format(time.RFC3339)
			data["scope"] = token.Scope
		}
		jsonOut, _ := cmd.Flags().GetBool("json")
		if jsonOut {
			return cli.WriteSuccess(cmd.OutOrStdout(), data)
		}
		if logged {
			_, err = fmt.Fprint(cmd.OutOrStdout(), cli.Human(cmd).Localizer.Humanf("Profile %s is logged in.\n", profile))
		} else {
			_, err = fmt.Fprint(cmd.OutOrStdout(), cli.Human(cmd).Localizer.Humanf("Profile %s is not logged in.\n", profile))
		}
		return err
	}}
}

func confirmationError(err error) *cli.Error {
	if errors.Is(err, prompt.ErrNonInteractive) {
		return &cli.Error{Code: "confirmation_required", Message: "confirmation requires an interactive terminal; pass --yes to continue", ExitCode: cli.ExitUsage}
	}
	if errors.Is(err, prompt.ErrAborted) {
		return &cli.Error{Code: "operation_cancelled", Message: "operation cancelled", ExitCode: cli.ExitConflict}
	}
	return &cli.Error{Code: "confirmation_failed", Message: "confirmation could not be read", ExitCode: cli.ExitFailure}
}
func safe(code, message string, exit cli.ExitCode) *cli.Error {
	return &cli.Error{Code: code, Message: message, ExitCode: exit}
}
