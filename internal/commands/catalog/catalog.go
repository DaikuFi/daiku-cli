package catalog

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/DaikuFi/daiku-cli/generated/daikuv1"
	"github.com/DaikuFi/daiku-cli/internal/api"
	"github.com/DaikuFi/daiku-cli/internal/cli"
	"github.com/DaikuFi/daiku-cli/internal/output"
	"github.com/DaikuFi/daiku-cli/internal/profiles"
	"github.com/DaikuFi/daiku-cli/internal/prompt"
	openapi_types "github.com/oapi-codegen/runtime/types"
	"github.com/spf13/cobra"
)

type tokenSource interface {
	AccessToken(context.Context, string) (string, error)
}

type Module struct {
	Profiles   profiles.Store
	Tokens     tokenSource
	HTTPClient *http.Client
}

func New(p profiles.Store, tokens tokenSource, httpClient *http.Client) Module {
	return Module{Profiles: p, Tokens: tokens, HTTPClient: httpClient}
}

func (m Module) Register(root *cobra.Command) {
	root.AddCommand(m.households(), m.collection("account-groups", "account groups"), m.accounts(),
		m.collection("categories", "categories"), m.collection("tags", "tags"), m.collection("institutions", "institutions"))
}

type session struct {
	client *api.Client
	token  string
}

func (m Module) session(ctx context.Context) (session, error) {
	cfg, err := m.Profiles.Load()
	if err != nil {
		return session{}, commandError("profile_error", "profile configuration could not be read", cli.ExitFailure)
	}
	if cfg.Current == "" {
		return session{}, commandError("profile_required", "select a profile before using API commands", cli.ExitAuth)
	}
	token, err := m.Tokens.AccessToken(ctx, cfg.Current)
	if err != nil {
		return session{}, commandError("authentication_required", "authenticate the active profile before using API commands", cli.ExitAuth)
	}
	client, err := api.New(api.Config{BaseURL: cfg.Profiles[cfg.Current].APIURL, HTTPClient: m.HTTPClient, UserAgent: "daiku-cli"})
	if err != nil {
		return session{}, commandError("profile_error", "the active profile has an invalid API URL", cli.ExitFailure)
	}
	return session{client, token}, nil
}

func (s session) do(ctx context.Context, method, path string, body, out any) error {
	if err := s.client.Do(ctx, method, path, s.token, body, out); err != nil {
		return mapAPIError(err)
	}
	return nil
}

func (m Module) households() *cobra.Command {
	cmd := &cobra.Command{Use: "households", Short: "Manage households", Args: cli.UsageArgs(cobra.NoArgs)}
	cmd.AddCommand(m.householdList(), m.householdGet(), m.householdCreate(), m.householdUpdate(), m.householdDelete(), m.householdReorder())
	return cmd
}

func (m Module) householdList() *cobra.Command {
	return run("list", "List households", cobra.NoArgs, func(cmd *cobra.Command, _ []string) error {
		var rows []map[string]any
		if err := m.call(cmd, http.MethodGet, "households/", nil, &rows); err != nil {
			return err
		}
		sortItems(rows)
		return emitList(cmd, "households", rows)
	})
}

func (m Module) householdGet() *cobra.Command {
	return run("get <household>", "Get a household", cobra.ExactArgs(1), func(cmd *cobra.Command, args []string) error {
		id, err := m.resolve(cmd, "households/", args[0])
		if err != nil {
			return err
		}
		var item map[string]any
		if err = m.call(cmd, http.MethodGet, "households/"+id+"/", nil, &item); err != nil {
			return err
		}
		return emitOne(cmd, item)
	})
}

func (m Module) householdCreate() *cobra.Command {
	var name, currency, emoji string
	cmd := run("create", "Create a household", cobra.NoArgs, func(cmd *cobra.Command, _ []string) error {
		body := daikuv1.HouseholdRequest{Name: name}
		if currency != "" {
			value := daikuv1.DisplayCurrency3e8Enum(currency)
			body.DisplayCurrency = &value
		}
		if emoji != "" {
			body.Emoji = &emoji
		}
		var item map[string]any
		if err := m.call(cmd, http.MethodPost, "households/", body, &item); err != nil {
			return err
		}
		return emitOne(cmd, item)
	})
	cmd.Flags().StringVar(&name, "name", "", "household name")
	cmd.Flags().StringVar(&currency, "display-currency", "", "display currency")
	cmd.Flags().StringVar(&emoji, "emoji", "", "household emoji")
	_ = cmd.MarkFlagRequired("name")
	return cmd
}

func (m Module) householdUpdate() *cobra.Command {
	var name, currency, emoji string
	cmd := run("update <household>", "Update a household", cobra.ExactArgs(1), func(cmd *cobra.Command, args []string) error {
		if name == "" && currency == "" && emoji == "" {
			return usage("provide at least one field to update")
		}
		id, err := m.resolve(cmd, "households/", args[0])
		if err != nil {
			return err
		}
		body := daikuv1.PatchedHouseholdRequest{}
		if name != "" {
			body.Name = &name
		}
		if emoji != "" {
			body.Emoji = &emoji
		}
		if currency != "" {
			value := daikuv1.DisplayCurrency3e8Enum(currency)
			body.DisplayCurrency = &value
		}
		var item map[string]any
		if err = m.call(cmd, http.MethodPatch, "households/"+id+"/", body, &item); err != nil {
			return err
		}
		return emitOne(cmd, item)
	})
	cmd.Flags().StringVar(&name, "name", "", "household name")
	cmd.Flags().StringVar(&currency, "display-currency", "", "display currency")
	cmd.Flags().StringVar(&emoji, "emoji", "", "household emoji")
	return cmd
}

func (m Module) householdDelete() *cobra.Command {
	var yes bool
	cmd := run("delete <household>", "Delete a household", cobra.ExactArgs(1), func(cmd *cobra.Command, args []string) error {
		id, err := m.resolve(cmd, "households/", args[0])
		if err != nil {
			return err
		}
		if err = confirm(cmd, yes, "Delete household "+id+"."); err != nil {
			return err
		}
		if err = m.call(cmd, http.MethodDelete, "households/"+id+"/", nil, nil); err != nil {
			return err
		}
		return emitOne(cmd, map[string]any{"deleted": true, "id": id})
	})
	cmd.Flags().BoolVar(&yes, "yes", false, "skip the interactive confirmation")
	return cmd
}

func (m Module) householdReorder() *cobra.Command {
	var ids []string
	cmd := run("reorder", "Reorder households", cobra.NoArgs, func(cmd *cobra.Command, _ []string) error {
		if len(ids) == 0 {
			return usage("provide at least one --id")
		}
		items := make([]daikuv1.HouseholdReorderItemRequest, len(ids))
		for i, id := range ids {
			order := i
			items[i] = daikuv1.HouseholdReorderItemRequest{Id: id, SortOrder: &order}
		}
		var result any
		if err := m.call(cmd, http.MethodPost, "households/reorder/", items, &result); err != nil {
			return err
		}
		return emitOne(cmd, result)
	})
	cmd.Flags().StringSliceVar(&ids, "id", nil, "household IDs in desired order")
	return cmd
}

type resourceSpec struct{ path, label string }

func (m Module) collection(name, label string) *cobra.Command {
	spec := resourceSpec{name, label}
	cmd := &cobra.Command{Use: name, Short: "Manage " + label, Args: cli.UsageArgs(cobra.NoArgs)}
	cmd.AddCommand(m.resourceList(spec), m.resourceCreate(spec), m.resourceUpdate(spec), m.resourceDelete(spec))
	if name == "account-groups" || name == "categories" {
		cmd.AddCommand(m.resourceReorder(spec))
	}
	return cmd
}

func scopedPath(household, resource string) string {
	return "households/" + household + "/" + resource + "/"
}

func addHouseholdFlag(cmd *cobra.Command, target *string) {
	cmd.Flags().StringVar(target, "household", "", "household ID")
	_ = cmd.MarkFlagRequired("household")
}

func (m Module) resourceList(spec resourceSpec) *cobra.Command {
	var household string
	cmd := run("list", "List "+spec.label, cobra.NoArgs, func(cmd *cobra.Command, _ []string) error {
		var rows []map[string]any
		if err := m.call(cmd, http.MethodGet, scopedPath(household, spec.path), nil, &rows); err != nil {
			return err
		}
		sortItems(rows)
		return emitList(cmd, spec.path, rows)
	})
	addHouseholdFlag(cmd, &household)
	return cmd
}

func (m Module) resourceCreate(spec resourceSpec) *cobra.Command {
	var household, name, emoji, color, domain, parent string
	cmd := run("create", "Create "+singular(spec.label), cobra.NoArgs, func(cmd *cobra.Command, _ []string) error {
		var body any
		switch spec.path {
		case "account-groups":
			body = daikuv1.PublicAccountGroupRequest{Name: name, Emoji: optional(emoji)}
		case "categories":
			body = daikuv1.PublicCategoryRequest{Name: name, Emoji: optional(emoji), Parent: optional(parent)}
		case "tags":
			body = daikuv1.PublicTagRequest{Name: name, Color: optional(color)}
		case "institutions":
			body = daikuv1.PublicFinancialInstitutionRequest{Name: name, Domain: optional(domain)}
		}
		var item map[string]any
		if err := m.call(cmd, http.MethodPost, scopedPath(household, spec.path), body, &item); err != nil {
			return err
		}
		return emitOne(cmd, item)
	})
	addHouseholdFlag(cmd, &household)
	cmd.Flags().StringVar(&name, "name", "", "name")
	_ = cmd.MarkFlagRequired("name")
	cmd.Flags().StringVar(&emoji, "emoji", "", "emoji")
	cmd.Flags().StringVar(&color, "color", "", "color")
	cmd.Flags().StringVar(&domain, "domain", "", "domain")
	cmd.Flags().StringVar(&parent, "parent", "", "parent category ID")
	return cmd
}

func (m Module) resourceUpdate(spec resourceSpec) *cobra.Command {
	var household, name, emoji, color, domain, parent string
	cmd := run("update <resource>", "Update "+singular(spec.label), cobra.ExactArgs(1), func(cmd *cobra.Command, args []string) error {
		if name == "" && emoji == "" && color == "" && domain == "" && parent == "" {
			return usage("provide at least one field to update")
		}
		base := scopedPath(household, spec.path)
		id, err := m.resolve(cmd, base, args[0])
		if err != nil {
			return err
		}
		var body any
		switch spec.path {
		case "account-groups":
			body = daikuv1.PatchedPublicAccountGroupRequest{Name: optional(name), Emoji: optional(emoji)}
		case "categories":
			body = daikuv1.PatchedPublicCategoryRequest{Name: optional(name), Emoji: optional(emoji), Parent: optional(parent)}
		case "tags":
			body = daikuv1.PatchedPublicTagRequest{Name: optional(name), Color: optional(color)}
		case "institutions":
			body = daikuv1.PatchedPublicFinancialInstitutionRequest{Name: optional(name), Domain: optional(domain)}
		}
		var item map[string]any
		if err = m.call(cmd, http.MethodPatch, base+id+"/", body, &item); err != nil {
			return err
		}
		return emitOne(cmd, item)
	})
	addHouseholdFlag(cmd, &household)
	cmd.Flags().StringVar(&name, "name", "", "name")
	cmd.Flags().StringVar(&emoji, "emoji", "", "emoji")
	cmd.Flags().StringVar(&color, "color", "", "color")
	cmd.Flags().StringVar(&domain, "domain", "", "domain")
	cmd.Flags().StringVar(&parent, "parent", "", "parent category ID")
	return cmd
}

func (m Module) resourceDelete(spec resourceSpec) *cobra.Command {
	var household string
	var yes bool
	cmd := run("delete <resource>", "Delete "+singular(spec.label), cobra.ExactArgs(1), func(cmd *cobra.Command, args []string) error {
		base := scopedPath(household, spec.path)
		id, err := m.resolve(cmd, base, args[0])
		if err != nil {
			return err
		}
		if err = confirm(cmd, yes, "Delete "+id+"."); err != nil {
			return err
		}
		if err = m.call(cmd, http.MethodDelete, base+id+"/", nil, nil); err != nil {
			return err
		}
		return emitOne(cmd, map[string]any{"deleted": true, "id": id})
	})
	addHouseholdFlag(cmd, &household)
	cmd.Flags().BoolVar(&yes, "yes", false, "skip the interactive confirmation")
	return cmd
}

func (m Module) resourceReorder(spec resourceSpec) *cobra.Command {
	var household string
	var ids []string
	cmd := run("reorder", "Reorder "+spec.label, cobra.NoArgs, func(cmd *cobra.Command, _ []string) error {
		if len(ids) == 0 {
			return usage("provide at least one --id")
		}
		var items any
		switch spec.path {
		case "account-groups":
			values := make([]daikuv1.AccountGroupReorderItemRequest, len(ids))
			for i, id := range ids {
				order := i
				values[i] = daikuv1.AccountGroupReorderItemRequest{Id: id, SortOrder: &order}
			}
			items = values
		case "categories":
			values := make([]daikuv1.CategoryReorderItemRequest, len(ids))
			for i, id := range ids {
				order := i
				values[i] = daikuv1.CategoryReorderItemRequest{Id: id, SortOrder: &order}
			}
			items = values
		case "accounts":
			values := make([]daikuv1.AccountReorderItemRequest, len(ids))
			for i, id := range ids {
				order := i
				values[i] = daikuv1.AccountReorderItemRequest{Id: id, SortOrder: &order}
			}
			items = values
		}
		var result any
		if err := m.call(cmd, http.MethodPost, scopedPath(household, spec.path)+"reorder/", items, &result); err != nil {
			return err
		}
		return emitOne(cmd, result)
	})
	addHouseholdFlag(cmd, &household)
	cmd.Flags().StringSliceVar(&ids, "id", nil, "resource IDs in desired order")
	return cmd
}

func (m Module) accounts() *cobra.Command {
	cmd := &cobra.Command{Use: "accounts", Short: "Manage accounts", Args: cli.UsageArgs(cobra.NoArgs)}
	cmd.AddCommand(m.accountList(), m.accountCreate(), m.accountUpdate(), m.accountArchive(), m.accountUnarchive(), m.accountAdjust(), m.accountReorder())
	return cmd
}

func (m Module) accountList() *cobra.Command {
	var hh string
	var archived string
	cmd := run("list", "List accounts", cobra.NoArgs, func(cmd *cobra.Command, _ []string) error {
		path := scopedPath(hh, "accounts")
		if archived != "" {
			path += "?archived=" + archived
		}
		var rows []map[string]any
		if err := m.call(cmd, http.MethodGet, path, nil, &rows); err != nil {
			return err
		}
		sortItems(rows)
		return emitList(cmd, "accounts", rows)
	})
	addHouseholdFlag(cmd, &hh)
	cmd.Flags().StringVar(&archived, "archived", "", "include archived accounts: true or all")
	return cmd
}

func (m Module) accountCreate() *cobra.Command {
	var hh, name, currency, kind, group, institution, balance string
	cmd := run("create", "Create an account", cobra.NoArgs, func(cmd *cobra.Command, _ []string) error {
		body := daikuv1.PublicAccountWriteRequest{Name: &name, Group: optional(group), Institution: optional(institution), OpeningBalance: optional(balance)}
		if currency != "" {
			v := daikuv1.Currency595Enum(currency)
			body.Currency = &v
		}
		if kind != "" {
			v := daikuv1.PublicAccountWriteAccountTypeEnum(kind)
			body.AccountType = &v
		}
		var item map[string]any
		if err := m.call(cmd, http.MethodPost, scopedPath(hh, "accounts"), body, &item); err != nil {
			return err
		}
		return emitOne(cmd, item)
	})
	addHouseholdFlag(cmd, &hh)
	cmd.Flags().StringVar(&name, "name", "", "account name")
	cmd.Flags().StringVar(&currency, "currency", "", "currency")
	cmd.Flags().StringVar(&kind, "type", "", "account type")
	cmd.Flags().StringVar(&group, "group", "", "account group ID")
	cmd.Flags().StringVar(&institution, "institution", "", "institution ID")
	cmd.Flags().StringVar(&balance, "opening-balance", "", "opening balance")
	_ = cmd.MarkFlagRequired("name")
	return cmd
}

func (m Module) accountUpdate() *cobra.Command {
	var hh, name, group, institution string
	cmd := run("update <account>", "Update an account", cobra.ExactArgs(1), func(cmd *cobra.Command, args []string) error {
		if name == "" && group == "" && institution == "" {
			return usage("provide at least one field to update")
		}
		base := scopedPath(hh, "accounts")
		id, err := m.resolve(cmd, base, args[0])
		if err != nil {
			return err
		}
		body := daikuv1.PatchedPublicAccountWriteRequest{Name: optional(name), Group: optional(group), Institution: optional(institution)}
		var item map[string]any
		if err = m.call(cmd, http.MethodPatch, base+id+"/", body, &item); err != nil {
			return err
		}
		return emitOne(cmd, item)
	})
	addHouseholdFlag(cmd, &hh)
	cmd.Flags().StringVar(&name, "name", "", "account name")
	cmd.Flags().StringVar(&group, "group", "", "account group ID")
	cmd.Flags().StringVar(&institution, "institution", "", "institution ID")
	return cmd
}

func (m Module) accountArchive() *cobra.Command {
	return m.accountAction("archive", http.MethodDelete, "", true)
}
func (m Module) accountUnarchive() *cobra.Command {
	return m.accountAction("unarchive", http.MethodPost, "unarchive/", false)
}
func (m Module) accountAction(name, method, suffix string, destructive bool) *cobra.Command {
	var hh string
	var yes bool
	cmd := run(name+" <account>", strings.ToUpper(name[:1])+name[1:]+" an account", cobra.ExactArgs(1), func(cmd *cobra.Command, args []string) error {
		base := scopedPath(hh, "accounts")
		id, err := m.resolve(cmd, base+"?archived=all", args[0])
		if err != nil {
			return err
		}
		if destructive {
			if err = confirm(cmd, yes, "Archive account "+id+"."); err != nil {
				return err
			}
		}
		var item map[string]any
		if err = m.call(cmd, method, base+id+"/"+suffix, nil, &item); err != nil {
			return err
		}
		return emitOne(cmd, item)
	})
	addHouseholdFlag(cmd, &hh)
	if destructive {
		cmd.Flags().BoolVar(&yes, "yes", false, "skip the interactive confirmation")
	}
	return cmd
}

func (m Module) accountAdjust() *cobra.Command {
	var hh, target, date, note string
	var yes bool
	cmd := run("adjust <account>", "Adjust an account balance", cobra.ExactArgs(1), func(cmd *cobra.Command, args []string) error {
		base := scopedPath(hh, "accounts")
		id, err := m.resolve(cmd, base+"?archived=all", args[0])
		if err != nil {
			return err
		}
		if err = confirm(cmd, yes, "Adjust account "+id+" balance."); err != nil {
			return err
		}
		body := daikuv1.AccountAdjustRequestRequest{TargetBalance: target, Note: optional(note)}
		if date != "" {
			parsed, parseErr := time.Parse("2006-01-02", date)
			if parseErr != nil {
				return usage("date must use YYYY-MM-DD")
			}
			value := openapi_types.Date{Time: parsed}
			body.Date = &value
		}
		var item map[string]any
		if err = m.call(cmd, http.MethodPost, base+id+"/adjust/", body, &item); err != nil {
			return err
		}
		return emitOne(cmd, item)
	})
	addHouseholdFlag(cmd, &hh)
	cmd.Flags().StringVar(&target, "target-balance", "", "target balance")
	cmd.Flags().StringVar(&date, "date", "", "adjustment date")
	cmd.Flags().StringVar(&note, "note", "", "adjustment note")
	cmd.Flags().BoolVar(&yes, "yes", false, "skip the interactive confirmation")
	_ = cmd.MarkFlagRequired("target-balance")
	return cmd
}

func (m Module) accountReorder() *cobra.Command {
	return m.resourceReorder(resourceSpec{"accounts", "accounts"})
}

func run(use, short string, args cobra.PositionalArgs, fn func(*cobra.Command, []string) error) *cobra.Command {
	return &cobra.Command{Use: use, Short: short, Args: cli.UsageArgs(args), RunE: fn}
}
func optional(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}
func singular(value string) string {
	if value == "categories" {
		return "category"
	}
	return strings.TrimSuffix(value, "s")
}
func usage(message string) *cli.Error { return commandError("usage_error", message, cli.ExitUsage) }
func commandError(code, message string, exit cli.ExitCode) *cli.Error {
	return &cli.Error{Code: code, Message: message, ExitCode: exit}
}

func (m Module) call(cmd *cobra.Command, method, path string, body, out any) error {
	s, err := m.session(cmd.Context())
	if err != nil {
		return err
	}
	return s.do(cmd.Context(), method, path, body, out)
}

func (m Module) resolve(cmd *cobra.Command, path, selector string) (string, error) {
	if strings.Contains(selector, "_") {
		return selector, nil
	}
	var rows []map[string]any
	if err := m.call(cmd, http.MethodGet, path, nil, &rows); err != nil {
		return "", err
	}
	matches := []string{}
	for _, item := range rows {
		if name, _ := item["name"].(string); strings.EqualFold(name, selector) {
			if id, _ := item["id"].(string); id != "" {
				matches = append(matches, id)
			}
		}
	}
	if len(matches) == 0 {
		return "", commandError("not_found", "the requested resource was not found", cli.ExitNotFound)
	}
	if len(matches) > 1 {
		sort.Strings(matches)
		return "", &cli.Error{Code: "ambiguous_resource", Message: "the resource name is ambiguous; use an ID", ExitCode: cli.ExitConflict, Details: map[string]any{"ids": matches}}
	}
	return matches[0], nil
}

func mapAPIError(err error) error {
	var e *api.Error
	if !errors.As(err, &e) {
		return commandError("api_error", "the Daiku API request failed", cli.ExitFailure)
	}
	exit := cli.ExitFailure
	switch e.StatusCode {
	case 401:
		exit = cli.ExitAuth
	case 403:
		exit = cli.ExitForbidden
	case 404:
		exit = cli.ExitNotFound
	case 409:
		exit = cli.ExitConflict
	case 429:
		exit = cli.ExitUnavailable
	}
	if e.StatusCode >= 500 {
		exit = cli.ExitUnavailable
	}
	return commandError(e.Code, e.Message, exit)
}

func confirm(cmd *cobra.Command, yes bool, message string) error {
	if yes {
		return nil
	}
	h := cli.Human(cmd)
	prompter := prompt.Prompter{In: cmd.InOrStdin(), Out: cmd.ErrOrStderr(), Localize: h.Localizer, Terminal: h.Interactive && !h.JSON}
	err := prompter.ConfirmDestructive(h.Localizer.Human(message))
	if errors.Is(err, prompt.ErrNonInteractive) {
		return commandError("confirmation_required", "confirmation requires an interactive terminal; pass --yes to continue", cli.ExitUsage)
	}
	if errors.Is(err, prompt.ErrAborted) {
		return commandError("operation_cancelled", "operation cancelled", cli.ExitConflict)
	}
	if err != nil {
		return commandError("confirmation_failed", "confirmation could not be read", cli.ExitFailure)
	}
	return nil
}

func sortItems(items []map[string]any) {
	sort.SliceStable(items, func(i, j int) bool {
		ni, _ := items[i]["name"].(string)
		nj, _ := items[j]["name"].(string)
		if ni == nj {
			return fmt.Sprint(items[i]["id"]) < fmt.Sprint(items[j]["id"])
		}
		return strings.ToLower(ni) < strings.ToLower(nj)
	})
}
func emitOne(cmd *cobra.Command, item any) error {
	jsonOut, _ := cmd.Flags().GetBool("json")
	if jsonOut {
		return cli.WriteSuccess(cmd.OutOrStdout(), item)
	}
	_, err := fmt.Fprintln(cmd.OutOrStdout(), humanValue(item))
	return err
}
func emitList(cmd *cobra.Command, key string, items []map[string]any) error {
	jsonOut, _ := cmd.Flags().GetBool("json")
	if jsonOut {
		return cli.WriteSuccess(cmd.OutOrStdout(), map[string]any{key: items})
	}
	h := cli.Human(cmd)
	rows := make([]output.Row, 0, len(items))
	for _, item := range items {
		rows = append(rows, output.Row{{Label: "ID", Value: fmt.Sprint(item["id"])}, {Label: h.Localizer.Human("NAME"), Value: fmt.Sprint(item["name"])}})
	}
	return (output.Renderer{Writer: cmd.OutOrStdout(), Localize: h.Localizer, Terminal: h.Terminal, Width: h.Width, NoColor: h.NoColor}).Table(rows)
}
func humanValue(value any) string {
	if item, ok := value.(map[string]any); ok {
		if name, ok := item["name"].(string); ok {
			if id, ok := item["id"].(string); ok {
				return name + " (" + id + ")"
			}
			return name
		}
		keys := make([]string, 0, len(item))
		for k := range item {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		parts := make([]string, 0, len(keys))
		for _, k := range keys {
			parts = append(parts, k+"="+fmt.Sprint(item[k]))
		}
		return strings.Join(parts, " ")
	}
	return fmt.Sprint(value)
}
