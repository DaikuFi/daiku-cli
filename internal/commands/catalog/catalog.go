package catalog

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/DaikuFi/daiku-cli/generated/daikuv1"
	"github.com/DaikuFi/daiku-cli/internal/api"
	"github.com/DaikuFi/daiku-cli/internal/cli"
	"github.com/DaikuFi/daiku-cli/internal/currency"
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
	cmd.AddCommand(m.householdList(), m.householdGet(), m.householdCreate(), m.householdUpdate(), m.householdMode(), m.householdDelete(), m.householdReorder())
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
			value, err := displayCurrency(currency)
			if err != nil {
				return err
			}
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
		if !cmd.Flags().Changed("name") && !cmd.Flags().Changed("display-currency") && !cmd.Flags().Changed("emoji") {
			return usage("provide at least one field to update")
		}
		id, err := m.resolve(cmd, "households/", args[0])
		if err != nil {
			return err
		}
		// The generated PATCH type has a nullable field without omitempty. A
		// presence-aware map keeps omitted values omitted instead of silently
		// sending first_accountable_date=null.
		body := map[string]any{}
		if cmd.Flags().Changed("name") {
			body["name"] = name
		}
		if cmd.Flags().Changed("emoji") {
			body["emoji"] = emoji
		}
		if cmd.Flags().Changed("display-currency") {
			value, parseErr := displayCurrency(currency)
			if parseErr != nil {
				return parseErr
			}
			body["display_currency"] = value
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

func (m Module) householdMode() *cobra.Command {
	var usesAccounts bool
	cmd := run("mode <household>", "Set household mode", cobra.ExactArgs(1), func(cmd *cobra.Command, args []string) error {
		id, err := m.resolve(cmd, "households/", args[0])
		if err != nil {
			return err
		}
		body := daikuv1.HouseholdModeRequest{UsesAccounts: usesAccounts}
		var item map[string]any
		if err = m.call(cmd, http.MethodPost, "households/"+id+"/mode/", body, &item); err != nil {
			return err
		}
		return emitOne(cmd, item)
	})
	cmd.Flags().BoolVar(&usesAccounts, "uses-accounts", false, "account mode (required)")
	_ = cmd.MarkFlagRequired("uses-accounts")
	return cmd
}

func (m Module) householdDelete() *cobra.Command {
	var yes bool
	cmd := run("delete <household>", "Delete a household", cobra.ExactArgs(1), func(cmd *cobra.Command, args []string) error {
		id, err := m.resolve(cmd, "households/", args[0])
		if err != nil {
			return err
		}
		if err = confirm(cmd, yes, "Delete household %s.", id); err != nil {
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
	var household, name, emoji, color, domain, country, parent string
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
			value := daikuv1.PublicFinancialInstitutionRequest{Name: name, Domain: optional(domain)}
			if country != "" {
				parsed, err := institutionCountry(country)
				if err != nil {
					return err
				}
				var union daikuv1.PublicFinancialInstitutionRequest_Country
				if err := union.FromCountry806Enum(parsed); err != nil {
					return commandError("invalid_request", "country could not be encoded", cli.ExitFailure)
				}
				value.Country = &union
			}
			body = value
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
	switch spec.path {
	case "account-groups":
		cmd.Flags().StringVar(&emoji, "emoji", "", "emoji")
	case "categories":
		cmd.Flags().StringVar(&emoji, "emoji", "", "emoji")
		cmd.Flags().StringVar(&parent, "parent", "", "parent category ID")
	case "tags":
		cmd.Flags().StringVar(&color, "color", "", "color")
	case "institutions":
		cmd.Flags().StringVar(&domain, "domain", "", "domain")
		cmd.Flags().StringVar(&country, "country", "", "ISO country code")
	}
	return cmd
}

func (m Module) resourceUpdate(spec resourceSpec) *cobra.Command {
	var household, name, emoji, color, domain, country, parent string
	var clearParent bool
	cmd := run("update <resource>", "Update "+singular(spec.label), cobra.ExactArgs(1), func(cmd *cobra.Command, args []string) error {
		if parent != "" && clearParent {
			return usage("--parent and --clear-parent cannot be used together")
		}
		fields := []string{"name", "emoji", "color", "domain", "country", "parent"}
		changed := false
		for _, flag := range fields {
			if cmd.Flags().Lookup(flag) != nil && cmd.Flags().Changed(flag) {
				changed = true
			}
		}
		if !changed && !clearParent {
			return usage("provide at least one field to update")
		}
		base := scopedPath(household, spec.path)
		id, err := m.resolve(cmd, base, args[0])
		if err != nil {
			return err
		}
		body := map[string]any{}
		if spec.path == "institutions" && cmd.Flags().Changed("country") {
			parsed, parseErr := institutionCountry(country)
			if parseErr != nil {
				return parseErr
			}
			country = string(parsed)
		}
		for _, pair := range []struct{ flag, key, value string }{{"name", "name", name}, {"emoji", "emoji", emoji}, {"color", "color", color}, {"domain", "domain", domain}, {"country", "country", country}, {"parent", "parent", parent}} {
			if cmd.Flags().Lookup(pair.flag) != nil && cmd.Flags().Changed(pair.flag) {
				body[pair.key] = pair.value
			}
		}
		if clearParent {
			body["parent"] = nil
		}
		var item map[string]any
		if err = m.call(cmd, http.MethodPatch, base+id+"/", body, &item); err != nil {
			return err
		}
		return emitOne(cmd, item)
	})
	addHouseholdFlag(cmd, &household)
	cmd.Flags().StringVar(&name, "name", "", "name")
	switch spec.path {
	case "account-groups":
		cmd.Flags().StringVar(&emoji, "emoji", "", "emoji")
	case "categories":
		cmd.Flags().StringVar(&emoji, "emoji", "", "emoji")
		cmd.Flags().StringVar(&parent, "parent", "", "parent category ID")
		cmd.Flags().BoolVar(&clearParent, "clear-parent", false, "clear the parent category")
	case "tags":
		cmd.Flags().StringVar(&color, "color", "", "color")
	case "institutions":
		cmd.Flags().StringVar(&domain, "domain", "", "domain")
		cmd.Flags().StringVar(&country, "country", "", "ISO country code")
	}
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
		if err = confirm(cmd, yes, "Delete resource %s.", id); err != nil {
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
			// CategoryReorderItemRequest.Parent is optional in the API contract but
			// its generated JSON tag lacks omitempty. Using that type here would send
			// parent:null and explicitly detach every reordered subcategory.
			values := make([]struct {
				ID        string `json:"id"`
				SortOrder int    `json:"sort_order"`
			}, len(ids))
			for i, id := range ids {
				values[i].ID = id
				values[i].SortOrder = i
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
	var hh, name, currency, kind, group, institution, balance, emoji, number, holder string
	var isDefault bool
	cmd := run("create", "Create an account", cobra.NoArgs, func(cmd *cobra.Command, _ []string) error {
		body := daikuv1.PublicAccountWriteRequest{Name: &name, Group: optional(group), Institution: optional(institution), OpeningBalance: optional(balance), Emoji: optional(emoji), AccountNumber: optional(number), AccountHolder: optional(holder)}
		if cmd.Flags().Changed("is-default") {
			body.IsDefault = &isDefault
		}
		if currency != "" {
			v, err := accountCurrency(currency)
			if err != nil {
				return err
			}
			body.Currency = &v
		}
		if kind != "" {
			v, err := accountType(kind)
			if err != nil {
				return err
			}
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
	cmd.Flags().StringVar(&emoji, "emoji", "", "emoji")
	cmd.Flags().StringVar(&number, "account-number", "", "account number")
	cmd.Flags().StringVar(&holder, "account-holder", "", "account holder")
	cmd.Flags().BoolVar(&isDefault, "is-default", false, "make this the default account")
	_ = cmd.MarkFlagRequired("name")
	return cmd
}

func (m Module) accountUpdate() *cobra.Command {
	var hh, name, group, institution, currency, kind, balance, emoji, number, holder string
	var isDefault, clearGroup, clearInstitution bool
	cmd := run("update <account>", "Update an account", cobra.ExactArgs(1), func(cmd *cobra.Command, args []string) error {
		if group != "" && clearGroup {
			return usage("--group and --clear-group cannot be used together")
		}
		if institution != "" && clearInstitution {
			return usage("--institution and --clear-institution cannot be used together")
		}
		changed := []string{"name", "group", "institution", "currency", "type", "opening-balance", "emoji", "account-number", "account-holder", "is-default"}
		has := false
		for _, flag := range changed {
			has = has || cmd.Flags().Changed(flag)
		}
		has = has || clearGroup || clearInstitution
		if !has {
			return usage("provide at least one field to update")
		}
		base := scopedPath(hh, "accounts")
		id, err := m.resolve(cmd, base, args[0])
		if err != nil {
			return err
		}
		body := map[string]any{}
		if cmd.Flags().Changed("currency") {
			parsed, parseErr := accountCurrency(currency)
			if parseErr != nil {
				return parseErr
			}
			currency = string(parsed)
		}
		if cmd.Flags().Changed("type") {
			parsed, parseErr := accountType(kind)
			if parseErr != nil {
				return parseErr
			}
			kind = string(parsed)
		}
		for flag, value := range map[string]any{"name": name, "group": group, "institution": institution, "currency": currency, "account_type": kind, "opening_balance": balance, "emoji": emoji, "account_number": number, "account_holder": holder, "is_default": isDefault} {
			flagName := strings.ReplaceAll(flag, "account_type", "type")
			flagName = strings.ReplaceAll(flagName, "opening_balance", "opening-balance")
			flagName = strings.ReplaceAll(flagName, "account_number", "account-number")
			flagName = strings.ReplaceAll(flagName, "account_holder", "account-holder")
			flagName = strings.ReplaceAll(flagName, "is_default", "is-default")
			if cmd.Flags().Changed(flagName) {
				body[flag] = value
			}
		}
		if clearGroup {
			body["group"] = nil
		}
		if clearInstitution {
			body["institution"] = nil
		}
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
	cmd.Flags().StringVar(&currency, "currency", "", "currency")
	cmd.Flags().StringVar(&kind, "type", "", "account type")
	cmd.Flags().StringVar(&balance, "opening-balance", "", "opening balance")
	cmd.Flags().StringVar(&emoji, "emoji", "", "emoji")
	cmd.Flags().StringVar(&number, "account-number", "", "account number")
	cmd.Flags().StringVar(&holder, "account-holder", "", "account holder")
	cmd.Flags().BoolVar(&isDefault, "is-default", false, "make this the default account")
	cmd.Flags().BoolVar(&clearGroup, "clear-group", false, "clear the account group")
	cmd.Flags().BoolVar(&clearInstitution, "clear-institution", false, "clear the institution")
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
			if err = confirm(cmd, yes, "Archive account %s.", id); err != nil {
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
		if err = confirm(cmd, yes, "Adjust account %s balance.", id); err != nil {
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

const publishedCountries = "|AD|AE|AF|AG|AI|AL|AM|AO|AQ|AR|AS|AT|AU|AW|AX|AZ|BA|BB|BD|BE|BF|BG|BH|BI|BJ|BL|BM|BN|BO|BQ|BR|BS|BT|BV|BW|BY|BZ|CA|CC|CD|CF|CG|CH|CI|CK|CL|CM|CN|CO|CR|CU|CV|CW|CX|CY|CZ|DE|DJ|DK|DM|DO|DZ|EC|EE|EG|EH|ER|ES|ET|FI|FJ|FK|FM|FO|FR|GA|GB|GD|GE|GF|GG|GH|GI|GL|GM|GN|GP|GQ|GR|GS|GT|GU|GW|GY|HK|HM|HN|HR|HT|HU|ID|IE|IL|IM|IN|IO|IQ|IR|IS|IT|JE|JM|JO|JP|KE|KG|KH|KI|KM|KN|KP|KR|KW|KY|KZ|LA|LB|LC|LI|LK|LR|LS|LT|LU|LV|LY|MA|MC|MD|ME|MF|MG|MH|MK|ML|MM|MN|MO|MP|MQ|MR|MS|MT|MU|MV|MW|MX|MY|MZ|NA|NC|NE|NF|NG|NI|NL|NO|NP|NR|NU|NZ|OM|PA|PE|PF|PG|PH|PK|PL|PM|PN|PR|PS|PT|PW|PY|QA|RE|RO|RS|RU|RW|SA|SB|SC|SD|SE|SG|SH|SI|SJ|SK|SL|SM|SN|SO|SR|SS|ST|SV|SX|SY|SZ|TC|TD|TF|TG|TH|TJ|TK|TL|TM|TN|TO|TR|TT|TV|TW|TZ|UA|UG|UM|US|UY|UZ|VA|VC|VE|VG|VI|VN|VU|WF|WS|YE|YT|ZA|ZM|ZW|"

var publishedAccountTypes = map[string]struct{}{
	"checking": {}, "savings": {}, "credit_card": {}, "loan": {}, "investment": {}, "cash": {}, "other": {},
}

func normalizeCurrency(value string) (string, error) {
	value, ok := currency.Normalize(value)
	if !ok {
		return "", usage("currency is not published by the Daiku API contract")
	}
	return value, nil
}
func accountCurrency(value string) (daikuv1.Currency595Enum, error) {
	normalized, err := normalizeCurrency(value)
	return daikuv1.Currency595Enum(normalized), err
}
func displayCurrency(value string) (daikuv1.DisplayCurrency3e8Enum, error) {
	normalized, err := normalizeCurrency(value)
	return daikuv1.DisplayCurrency3e8Enum(normalized), err
}
func institutionCountry(value string) (daikuv1.Country806Enum, error) {
	value = strings.ToUpper(strings.TrimSpace(value))
	if len(value) != 2 || !strings.Contains(publishedCountries, "|"+value+"|") {
		return "", usage("country is not published by the Daiku API contract")
	}
	return daikuv1.Country806Enum(value), nil
}
func accountType(value string) (daikuv1.PublicAccountWriteAccountTypeEnum, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	if _, ok := publishedAccountTypes[value]; !ok {
		return "", usage("account type is not published by the Daiku API contract")
	}
	return daikuv1.PublicAccountWriteAccountTypeEnum(value), nil
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

var resourceID = regexp.MustCompile(`^([a-z]+)_[0-9a-f]{32}$`)

func expectedPrefix(path string) string {
	switch {
	case strings.Contains(path, "/account-groups/"):
		return "agp"
	case strings.Contains(path, "/accounts/"):
		return "acc"
	case strings.Contains(path, "/categories/"):
		return "cat"
	case strings.Contains(path, "/tags/"):
		return "tag"
	case strings.Contains(path, "/institutions/"):
		return "inst"
	case strings.HasPrefix(path, "households/"):
		return "hsh"
	}
	return ""
}
func (m Module) resolve(cmd *cobra.Command, path, selector string) (string, error) {
	if match := resourceID.FindStringSubmatch(selector); match != nil {
		if match[1] != expectedPrefix(path) {
			return "", usage("resource ID has the wrong prefix")
		}
		return selector, nil
	}
	if prefix := expectedPrefix(path); prefix != "" && strings.HasPrefix(selector, prefix+"_") {
		return "", usage("resource ID has an invalid format")
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
	return &cli.Error{Code: e.Code, Message: e.Message, ExitCode: exit, Details: e.Details}
}

func confirm(cmd *cobra.Command, yes bool, format string, args ...any) error {
	if yes {
		return nil
	}
	h := cli.Human(cmd)
	prompter := prompt.Prompter{In: cmd.InOrStdin(), Out: cmd.ErrOrStderr(), Localize: h.Localizer, Terminal: h.Interactive && !h.JSON}
	err := prompter.ConfirmDestructive(h.Localizer.Humanf(format, args...))
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
