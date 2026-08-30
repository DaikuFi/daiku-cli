package budgets

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	daikuv1 "github.com/DaikuFi/daiku-cli/generated/daikuv1"
	authcore "github.com/DaikuFi/daiku-cli/internal/auth"
	"github.com/DaikuFi/daiku-cli/internal/cli"
	"github.com/DaikuFi/daiku-cli/internal/profiles"
	"github.com/DaikuFi/daiku-cli/internal/prompt"
	"github.com/spf13/cobra"
)

type API interface {
	Planned(context.Context, string, *daikuv1.DaikuHouseholdsHouseholdPkBudgetsPlannedGetParams) (*daikuv1.PlannedBudgets, error)
	Suggestions(context.Context, string, *daikuv1.DaikuHouseholdsHouseholdPkBudgetsSuggestionsGetParams) (*daikuv1.BudgetSuggestionsResponse, error)
	Summary(context.Context, string, *daikuv1.DaikuHouseholdsHouseholdPkBudgetsSummaryGetParams) (*daikuv1.BudgetSummary, error)
	List(context.Context, string) ([]daikuv1.CategoryBudget, error)
	Create(context.Context, string, daikuv1.CategoryBudgetRequest) (*daikuv1.CategoryBudget, error)
	Update(context.Context, string, string, Patch) (*daikuv1.CategoryBudget, error)
	Delete(context.Context, string, string) error
}

type Factory func(context.Context) (API, error)
type Module struct{ Factory Factory }
type Patch map[string]any

func New(store profiles.Store, manager *authcore.Manager) Module {
	return Module{Factory: func(ctx context.Context) (API, error) {
		cfg, err := store.Load()
		if err != nil || cfg.Current == "" {
			return nil, &cli.Error{Code: "profile_required", Message: "select and authenticate a profile first", ExitCode: cli.ExitAuth}
		}
		profile := cfg.Profiles[cfg.Current]
		token, err := manager.AccessToken(ctx, cfg.Current)
		if err != nil {
			return nil, &cli.Error{Code: "authentication_required", Message: "the active profile is not authenticated", ExitCode: cli.ExitAuth}
		}
		client, err := daikuv1.NewClientWithResponses(profile.APIURL, daikuv1.WithRequestEditorFn(func(_ context.Context, req *http.Request) error {
			req.Header.Set("Authorization", "Bearer "+token)
			return nil
		}))
		if err != nil {
			return nil, apiFailure()
		}
		return generatedAPI{client}, nil
	}}
}

func (m Module) Register(root *cobra.Command) {
	cmd := &cobra.Command{Use: "budgets", Short: "Inspect budget summaries and manage budget rules", Args: cli.UsageArgs(cobra.NoArgs), RunE: func(cmd *cobra.Command, _ []string) error { return cmd.Help() }}
	cmd.AddCommand(m.summary(), m.planned(), m.suggestions(), m.rules())
	root.AddCommand(cmd)
}

type periodFlags struct {
	household, currency string
	month, year         int
}

func addPeriodFlags(cmd *cobra.Command, f *periodFlags, month bool) {
	cmd.Flags().StringVar(&f.household, "household", "", "household ID")
	cmd.Flags().StringVar(&f.currency, "currency", "", "display currency: UYU, USD or EUR")
	if month {
		cmd.Flags().IntVar(&f.month, "month", 0, "month (1-12)")
	}
	cmd.Flags().IntVar(&f.year, "year", 0, "calendar year")
	_ = cmd.MarkFlagRequired("household")
}

func validatePeriod(f periodFlags, month bool) error {
	if month && (f.month < 1 || f.month > 12) {
		return usage("month must be between 1 and 12")
	}
	if f.year < 1 {
		return usage("year must be provided and greater than zero")
	}
	if f.currency != "UYU" && f.currency != "USD" && f.currency != "EUR" {
		return usage("currency must be UYU, USD or EUR")
	}
	return nil
}

func (m Module) summary() *cobra.Command {
	var f periodFlags
	cmd := &cobra.Command{Use: "summary", Short: "Show a monthly budget summary", Args: cli.UsageArgs(cobra.NoArgs), RunE: func(cmd *cobra.Command, _ []string) error {
		if err := validatePeriod(f, true); err != nil {
			return err
		}
		a, err := m.Factory(cmd.Context())
		if err != nil {
			return err
		}
		currency := daikuv1.DaikuHouseholdsHouseholdPkBudgetsSummaryGetParamsDisplayCurrency(f.currency)
		result, err := a.Summary(cmd.Context(), f.household, &daikuv1.DaikuHouseholdsHouseholdPkBudgetsSummaryGetParams{DisplayCurrency: &currency, Month: &f.month, Year: &f.year})
		return emit(cmd, result, "Budget summary for %02d/%d (%s).\n", f.month, f.year, f.currency, err)
	}}
	addPeriodFlags(cmd, &f, true)
	return cmd
}

func (m Module) planned() *cobra.Command {
	var f periodFlags
	cmd := &cobra.Command{Use: "planned", Short: "Show planned budgets for a year", Args: cli.UsageArgs(cobra.NoArgs), RunE: func(cmd *cobra.Command, _ []string) error {
		if err := validatePeriod(f, false); err != nil {
			return err
		}
		a, err := m.Factory(cmd.Context())
		if err != nil {
			return err
		}
		currency := daikuv1.DaikuHouseholdsHouseholdPkBudgetsPlannedGetParamsDisplayCurrency(f.currency)
		result, err := a.Planned(cmd.Context(), f.household, &daikuv1.DaikuHouseholdsHouseholdPkBudgetsPlannedGetParams{DisplayCurrency: &currency, Year: &f.year})
		return emit(cmd, result, "Planned budgets for %d (%s).\n", f.year, f.currency, err)
	}}
	addPeriodFlags(cmd, &f, false)
	return cmd
}

func (m Module) suggestions() *cobra.Command {
	var f periodFlags
	cmd := &cobra.Command{Use: "suggestions", Short: "Show budget suggestions for a month", Args: cli.UsageArgs(cobra.NoArgs), RunE: func(cmd *cobra.Command, _ []string) error {
		if err := validatePeriod(f, true); err != nil {
			return err
		}
		a, err := m.Factory(cmd.Context())
		if err != nil {
			return err
		}
		currency := daikuv1.DaikuHouseholdsHouseholdPkBudgetsSuggestionsGetParamsDisplayCurrency(f.currency)
		result, err := a.Suggestions(cmd.Context(), f.household, &daikuv1.DaikuHouseholdsHouseholdPkBudgetsSuggestionsGetParams{DisplayCurrency: &currency, Month: &f.month, Year: &f.year})
		return emit(cmd, result, "Budget suggestions for %02d/%d (%s).\n", f.month, f.year, f.currency, err)
	}}
	addPeriodFlags(cmd, &f, true)
	return cmd
}

func (m Module) rules() *cobra.Command {
	cmd := &cobra.Command{Use: "rules", Short: "Manage category budget rules", Args: cli.UsageArgs(cobra.NoArgs)}
	cmd.AddCommand(m.ruleList(), m.ruleCreate(), m.ruleUpdate(), m.ruleDelete())
	return cmd
}

func householdFlag(cmd *cobra.Command, value *string) {
	cmd.Flags().StringVar(value, "household", "", "household ID")
	_ = cmd.MarkFlagRequired("household")
}

func (m Module) ruleList() *cobra.Command {
	var household string
	cmd := &cobra.Command{Use: "list", Short: "List category budget rules", Args: cli.UsageArgs(cobra.NoArgs), RunE: func(cmd *cobra.Command, _ []string) error {
		a, err := m.Factory(cmd.Context())
		if err != nil {
			return err
		}
		result, err := a.List(cmd.Context(), household)
		return emit(cmd, map[string]any{"rules": result}, "%d budget rules.\n", len(result), err)
	}}
	householdFlag(cmd, &household)
	return cmd
}

type ruleFlags struct {
	household, category, amount, currency, scope string
	month, year                                  int
	clearMonth, clearYear                        bool
}

func addRuleFlags(cmd *cobra.Command, f *ruleFlags, create bool) {
	householdFlag(cmd, &f.household)
	cmd.Flags().StringVar(&f.category, "category", "", "category ID")
	cmd.Flags().StringVar(&f.amount, "amount", "", "budget amount")
	cmd.Flags().StringVar(&f.currency, "currency", "", "currency: UYU, USD or EUR")
	cmd.Flags().StringVar(&f.scope, "scope", "", "scope: monthly, yearly or month")
	cmd.Flags().IntVar(&f.month, "month", 0, "month (required for month scope)")
	cmd.Flags().IntVar(&f.year, "year", 0, "pinned year (required for month scope)")
	cmd.Flags().BoolVar(&f.clearMonth, "clear-month", false, "clear the pinned month")
	cmd.Flags().BoolVar(&f.clearYear, "clear-year", false, "clear the pinned year")
	if create {
		for _, name := range []string{"category", "amount", "currency", "scope"} {
			_ = cmd.MarkFlagRequired(name)
		}
	}
}
func validateRule(f ruleFlags, partial bool) error {
	if !partial && (f.category == "" || f.amount == "" || f.currency == "" || f.scope == "") {
		return usage("category, amount, currency and scope are required")
	}
	if f.currency != "" && !validCurrency(f.currency) {
		return usage("currency is not supported by the Daiku API contract")
	}
	if f.scope != "" && f.scope != "monthly" && f.scope != "yearly" && f.scope != "month" {
		return usage("scope must be monthly, yearly or month")
	}
	if f.scope == "month" && (f.month < 1 || f.month > 12 || f.year < 1) {
		return usage("month scope requires --month 1-12 and --year")
	}
	if (f.scope == "monthly" || f.scope == "yearly") && (f.month != 0 || f.year != 0) {
		return usage("month and year are only valid for month scope")
	}
	if (f.month != 0 && f.clearMonth) || (f.year != 0 && f.clearYear) {
		return usage("a field cannot be set and cleared together")
	}
	return nil
}
func validCurrency(value string) bool {
	for _, currency := range []string{"ARS", "BOB", "BRL", "CLP", "COP", "CRC", "DOP", "EUR", "GBP", "GTQ", "HNL", "MXN", "NIO", "PAB", "PEN", "PYG", "UI", "USD", "UYU", "VES"} {
		if value == currency {
			return true
		}
	}
	return false
}
func ruleRequest(f ruleFlags) daikuv1.CategoryBudgetRequest {
	currency := daikuv1.Currency3e8Enum(f.currency)
	scope := daikuv1.CategoryBudgetScopeEnum(f.scope)
	r := daikuv1.CategoryBudgetRequest{Category: f.category, Amount: f.amount, Currency: &currency, Scope: &scope}
	if f.month != 0 {
		r.Month = &f.month
	}
	if f.year != 0 {
		r.Year = &f.year
	}
	return r
}
func (m Module) ruleCreate() *cobra.Command {
	var f ruleFlags
	cmd := &cobra.Command{Use: "create", Short: "Create a category budget rule", Args: cli.UsageArgs(cobra.NoArgs), RunE: func(cmd *cobra.Command, _ []string) error {
		if err := validateRule(f, false); err != nil {
			return err
		}
		a, err := m.Factory(cmd.Context())
		if err != nil {
			return err
		}
		result, err := a.Create(cmd.Context(), f.household, ruleRequest(f))
		return emit(cmd, result, "Budget rule created.\n", err)
	}}
	addRuleFlags(cmd, &f, true)
	return cmd
}
func (m Module) ruleUpdate() *cobra.Command {
	var f ruleFlags
	cmd := &cobra.Command{Use: "update <id>", Short: "Update a category budget rule", Args: cli.UsageArgs(cobra.ExactArgs(1)), RunE: func(cmd *cobra.Command, args []string) error {
		if err := validateRule(f, true); err != nil {
			return err
		}
		if !cmd.Flags().Changed("category") && !cmd.Flags().Changed("amount") && !cmd.Flags().Changed("currency") && !cmd.Flags().Changed("scope") && !cmd.Flags().Changed("month") && !cmd.Flags().Changed("year") && !f.clearMonth && !f.clearYear {
			return usage("provide at least one field to update")
		}
		body := Patch{}
		if cmd.Flags().Changed("category") {
			body["category"] = f.category
		}
		if cmd.Flags().Changed("amount") {
			body["amount"] = f.amount
		}
		if cmd.Flags().Changed("currency") {
			body["currency"] = f.currency
		}
		if cmd.Flags().Changed("scope") {
			body["scope"] = f.scope
		}
		if cmd.Flags().Changed("month") {
			body["month"] = f.month
		}
		if cmd.Flags().Changed("year") {
			body["year"] = f.year
		}
		if f.clearMonth {
			body["month"] = nil
		}
		if f.clearYear {
			body["year"] = nil
		}
		a, err := m.Factory(cmd.Context())
		if err != nil {
			return err
		}
		result, err := a.Update(cmd.Context(), f.household, args[0], body)
		return emit(cmd, result, "Budget rule updated.\n", err)
	}}
	addRuleFlags(cmd, &f, false)
	return cmd
}
func (m Module) ruleDelete() *cobra.Command {
	var household string
	var yes bool
	cmd := &cobra.Command{Use: "delete <id>", Short: "Delete a category budget rule", Args: cli.UsageArgs(cobra.ExactArgs(1)), RunE: func(cmd *cobra.Command, args []string) error {
		if !yes {
			h := cli.Human(cmd)
			p := prompt.Prompter{In: cmd.InOrStdin(), Out: cmd.ErrOrStderr(), Localize: h.Localizer, Terminal: h.Interactive && !h.JSON}
			if err := p.ConfirmDestructive(h.Localizer.Humanf("Delete budget rule %s.", args[0])); err != nil {
				return confirmationError(err)
			}
		}
		a, err := m.Factory(cmd.Context())
		if err != nil {
			return err
		}
		err = a.Delete(cmd.Context(), household, args[0])
		return emit(cmd, map[string]any{"id": args[0], "deleted": err == nil}, "Budget rule deleted.\n", err)
	}}
	householdFlag(cmd, &household)
	cmd.Flags().BoolVar(&yes, "yes", false, "skip the interactive confirmation")
	return cmd
}

func emit(cmd *cobra.Command, data any, format string, args ...any) error {
	last := args[len(args)-1]
	if err, _ := last.(error); err != nil {
		return err
	}
	args = args[:len(args)-1]
	jsonOut, _ := cmd.Flags().GetBool("json")
	if jsonOut {
		return cli.WriteSuccess(cmd.OutOrStdout(), data)
	}
	_, err := fmt.Fprint(cmd.OutOrStdout(), cli.Human(cmd).Localizer.Humanf(format, args...))
	return err
}
func usage(message string) *cli.Error {
	return &cli.Error{Code: "usage_error", Message: message, ExitCode: cli.ExitUsage}
}
func apiFailure() *cli.Error {
	return &cli.Error{Code: "api_error", Message: "the Daiku API request failed", ExitCode: cli.ExitFailure}
}
func confirmationError(err error) *cli.Error {
	if errors.Is(err, prompt.ErrNonInteractive) {
		return &cli.Error{Code: "confirmation_required", Message: "confirmation requires an interactive terminal; pass --yes to continue", ExitCode: cli.ExitUsage}
	}
	if errors.Is(err, prompt.ErrAborted) {
		return &cli.Error{Code: "operation_cancelled", Message: "operation cancelled", ExitCode: cli.ExitConflict}
	}
	return apiFailure()
}

type generatedAPI struct{ c *daikuv1.ClientWithResponses }

func status(code int) error {
	if code >= 200 && code < 300 {
		return nil
	}
	if code == 401 {
		return &cli.Error{Code: "unauthorized", Message: "authentication is required", ExitCode: cli.ExitAuth}
	}
	if code == 403 {
		return &cli.Error{Code: "forbidden", Message: "your role does not allow this operation", ExitCode: cli.ExitForbidden}
	}
	if code == 404 {
		return &cli.Error{Code: "not_found", Message: "the requested resource was not found", ExitCode: cli.ExitNotFound}
	}
	return apiFailure()
}
func (a generatedAPI) Planned(ctx context.Context, h string, p *daikuv1.DaikuHouseholdsHouseholdPkBudgetsPlannedGetParams) (*daikuv1.PlannedBudgets, error) {
	r, e := a.c.DaikuHouseholdsHouseholdPkBudgetsPlannedGetWithResponse(ctx, h, p)
	if e != nil {
		return nil, apiFailure()
	}
	if e = status(r.StatusCode()); e != nil {
		return nil, e
	}
	return r.JSON200, nil
}
func (a generatedAPI) Suggestions(ctx context.Context, h string, p *daikuv1.DaikuHouseholdsHouseholdPkBudgetsSuggestionsGetParams) (*daikuv1.BudgetSuggestionsResponse, error) {
	r, e := a.c.DaikuHouseholdsHouseholdPkBudgetsSuggestionsGetWithResponse(ctx, h, p)
	if e != nil {
		return nil, apiFailure()
	}
	if e = status(r.StatusCode()); e != nil {
		return nil, e
	}
	return r.JSON200, nil
}
func (a generatedAPI) Summary(ctx context.Context, h string, p *daikuv1.DaikuHouseholdsHouseholdPkBudgetsSummaryGetParams) (*daikuv1.BudgetSummary, error) {
	r, e := a.c.DaikuHouseholdsHouseholdPkBudgetsSummaryGetWithResponse(ctx, h, p)
	if e != nil {
		return nil, apiFailure()
	}
	if e = status(r.StatusCode()); e != nil {
		return nil, e
	}
	return r.JSON200, nil
}
func (a generatedAPI) List(ctx context.Context, h string) ([]daikuv1.CategoryBudget, error) {
	r, e := a.c.DaikuHouseholdsHouseholdPkCategoryBudgetsGetWithResponse(ctx, h)
	if e != nil {
		return nil, apiFailure()
	}
	if e = status(r.StatusCode()); e != nil {
		return nil, e
	}
	return *r.JSON200, nil
}
func (a generatedAPI) Create(ctx context.Context, h string, b daikuv1.CategoryBudgetRequest) (*daikuv1.CategoryBudget, error) {
	r, e := a.c.DaikuHouseholdsHouseholdPkCategoryBudgetsPostWithResponse(ctx, h, b)
	if e != nil {
		return nil, apiFailure()
	}
	if e = status(r.StatusCode()); e != nil {
		return nil, e
	}
	return r.JSON201, nil
}
func (a generatedAPI) Update(ctx context.Context, h, id string, b Patch) (*daikuv1.CategoryBudget, error) {
	payload, e := json.Marshal(b)
	if e != nil {
		return nil, apiFailure()
	}
	r, e := a.c.DaikuHouseholdsHouseholdPkCategoryBudgetsIdPatchWithBodyWithResponse(ctx, h, id, "application/json", bytes.NewReader(payload))
	if e != nil {
		return nil, apiFailure()
	}
	if e = status(r.StatusCode()); e != nil {
		return nil, e
	}
	return r.JSON200, nil
}
func (a generatedAPI) Delete(ctx context.Context, h, id string) error {
	r, e := a.c.DaikuHouseholdsHouseholdPkCategoryBudgetsIdDeleteWithResponse(ctx, h, id)
	if e != nil {
		return apiFailure()
	}
	return status(r.StatusCode())
}
