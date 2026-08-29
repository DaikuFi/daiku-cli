package profile

import (
	"errors"
	"fmt"
	"sort"

	"github.com/DaikuFi/daiku-cli/internal/cli"
	"github.com/DaikuFi/daiku-cli/internal/credentials"
	"github.com/DaikuFi/daiku-cli/internal/profiles"
	"github.com/spf13/cobra"
)

type Module struct {
	Store       profiles.Store
	Credentials credentials.Store
}

func New(s profiles.Store, c credentials.Store) Module { return Module{s, c} }
func (m Module) Register(root *cobra.Command) {
	cmd := &cobra.Command{Use: "profile", Short: "Manage named Daiku profiles", Args: cli.UsageArgs(cobra.NoArgs)}
	cmd.AddCommand(m.add(), m.use(), m.list(), m.remove())
	root.AddCommand(cmd)
}
func (m Module) add() *cobra.Command {
	var apiURL string
	cmd := &cobra.Command{Use: "add <name>", Short: "Add a profile", Args: cli.UsageArgs(cobra.ExactArgs(1)), RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		if err := profiles.ValidateName(name); err != nil {
			return usage(err.Error())
		}
		value, err := profiles.NormalizeAPIURL(apiURL)
		if err != nil {
			return usage(err.Error())
		}
		cfg, err := m.Store.Load()
		if err != nil {
			return failure()
		}
		if _, ok := cfg.Profiles[name]; ok {
			return &cli.Error{Code: "profile_exists", Message: "profile already exists", ExitCode: cli.ExitConflict}
		}
		cfg.Profiles[name] = profiles.Profile{APIURL: value}
		if cfg.Current == "" {
			cfg.Current = name
		}
		if err = m.Store.Save(cfg); err != nil {
			return failure()
		}
		return emit(cmd, map[string]any{"name": name, "api_url": value, "current": cfg.Current == name}, fmt.Sprintf("Added profile %s.\n", name))
	}}
	cmd.Flags().StringVar(&apiURL, "api-url", "", "Daiku API URL")
	return cmd
}
func (m Module) use() *cobra.Command {
	return &cobra.Command{Use: "use <name>", Short: "Select the active profile", Args: cli.UsageArgs(cobra.ExactArgs(1)), RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := m.Store.Load()
		if err != nil {
			return failure()
		}
		if _, ok := cfg.Profiles[args[0]]; !ok {
			return &cli.Error{Code: "profile_not_found", Message: "profile does not exist", ExitCode: cli.ExitNotFound}
		}
		cfg.Current = args[0]
		if m.Store.Save(cfg) != nil {
			return failure()
		}
		return emit(cmd, map[string]any{"name": args[0], "current": true}, fmt.Sprintf("Using profile %s.\n", args[0]))
	}}
}
func (m Module) list() *cobra.Command {
	return &cobra.Command{Use: "list", Short: "List profiles", Args: cli.UsageArgs(cobra.NoArgs), RunE: func(cmd *cobra.Command, _ []string) error {
		cfg, err := m.Store.Load()
		if err != nil {
			return failure()
		}
		names := make([]string, 0, len(cfg.Profiles))
		for name := range cfg.Profiles {
			names = append(names, name)
		}
		sort.Strings(names)
		items := make([]map[string]any, 0, len(names))
		for _, name := range names {
			items = append(items, map[string]any{"name": name, "api_url": cfg.Profiles[name].APIURL, "current": name == cfg.Current})
		}
		jsonOut, _ := cmd.Flags().GetBool("json")
		if jsonOut {
			return cli.WriteSuccess(cmd.OutOrStdout(), map[string]any{"profiles": items})
		}
		for _, item := range items {
			mark := " "
			if item["current"].(bool) {
				mark = "*"
			}
			if _, err = fmt.Fprintf(cmd.OutOrStdout(), "%s %s\n", mark, item["name"]); err != nil {
				return err
			}
		}
		return nil
	}}
}
func (m Module) remove() *cobra.Command {
	return &cobra.Command{Use: "remove <name>", Short: "Remove a profile and its local credentials", Args: cli.UsageArgs(cobra.ExactArgs(1)), RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := m.Store.Load()
		if err != nil {
			return failure()
		}
		if _, ok := cfg.Profiles[args[0]]; !ok {
			return &cli.Error{Code: "profile_not_found", Message: "profile does not exist", ExitCode: cli.ExitNotFound}
		}
		if _, credentialErr := m.Credentials.Get(args[0]); credentialErr == nil {
			return &cli.Error{Code: "profile_authenticated", Message: "log out this profile before removing it", ExitCode: cli.ExitConflict}
		} else if !errors.Is(credentialErr, credentials.ErrNotFound) {
			return failure()
		}
		delete(cfg.Profiles, args[0])
		if cfg.Current == args[0] {
			cfg.Current = ""
		}
		if m.Store.Save(cfg) != nil {
			return failure()
		}
		return emit(cmd, map[string]any{"name": args[0], "removed": true}, fmt.Sprintf("Removed profile %s.\n", args[0]))
	}}
}
func emit(cmd *cobra.Command, data any, human string) error {
	jsonOut, _ := cmd.Flags().GetBool("json")
	if jsonOut {
		return cli.WriteSuccess(cmd.OutOrStdout(), data)
	}
	_, err := fmt.Fprint(cmd.OutOrStdout(), human)
	return err
}
func usage(message string) *cli.Error {
	return &cli.Error{Code: "usage_error", Message: message, ExitCode: cli.ExitUsage}
}
func failure() *cli.Error {
	return &cli.Error{Code: "profile_store_error", Message: "profile configuration could not be updated", ExitCode: cli.ExitFailure}
}
