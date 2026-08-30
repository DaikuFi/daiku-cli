package projections

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	daikuv1 "github.com/DaikuFi/daiku-cli/generated/daikuv1"
	authcore "github.com/DaikuFi/daiku-cli/internal/auth"
	"github.com/DaikuFi/daiku-cli/internal/cli"
	"github.com/DaikuFi/daiku-cli/internal/output"
	"github.com/DaikuFi/daiku-cli/internal/profiles"
	"github.com/DaikuFi/daiku-cli/internal/prompt"
	"github.com/oapi-codegen/runtime/types"
	"github.com/spf13/cobra"
)

type API interface {
	ScenarioList(context.Context, string) ([]daikuv1.ProjectionScenario, error)
	ScenarioCreate(context.Context, string, daikuv1.ProjectionScenarioRequest) (*daikuv1.ProjectionScenario, error)
	ScenarioUpdate(context.Context, string, string, scenarioPatch) (*daikuv1.ProjectionScenario, error)
	ScenarioDelete(context.Context, string, string) error
	Calculate(context.Context, string, string) (*daikuv1.ProjectionResult, error)
	Retirement(context.Context, string, string) (*daikuv1.RetirementResult, error)
	RuleList(context.Context, string) ([]daikuv1.ProjectionRule, error)
	RuleCreate(context.Context, string, daikuv1.ProjectionRuleRequest) (*daikuv1.ProjectionRule, error)
	RuleUpdate(context.Context, string, string, daikuv1.PatchedProjectionRuleRequest) (*daikuv1.ProjectionRule, error)
	RuleDelete(context.Context, string, string) error
	NetWorth(context.Context) (*daikuv1.NetWorthSeries, error)
	CurrencyExposure(context.Context) (*daikuv1.CurrencyExposure, error)
	Rates(context.Context, *daikuv1.DaikuExchangeRatesGetParams) ([]daikuv1.ExchangeRate, error)
}

type Factory func(context.Context) (API, error)
type Module struct{ Factory Factory }

// scenarioPatch preserves the three states required by PATCH: omitted, a
// concrete value, and explicit null. The generated request collapses omitted
// and null for birth_year because that nullable field lacks omitempty.
type scenarioPatch struct {
	Name         *string `json:"name,omitempty"`
	Color        *string `json:"color,omitempty"`
	BirthYear    **int   `json:"birth_year,omitempty"`
	DisplayOrder *int    `json:"display_order,omitempty"`
	IsActive     *bool   `json:"is_active,omitempty"`
}

func New(store profiles.Store, manager *authcore.Manager) Module {
	return Module{Factory: func(ctx context.Context) (API, error) {
		cfg, err := store.Load()
		if err != nil || cfg.Current == "" {
			return nil, &cli.Error{Code: "profile_required", Message: "select and authenticate a profile first", ExitCode: cli.ExitAuth}
		}
		profile, ok := cfg.Profiles[cfg.Current]
		if !ok {
			return nil, &cli.Error{Code: "profile_required", Message: "select and authenticate a profile first", ExitCode: cli.ExitAuth}
		}
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
	projections := &cobra.Command{Use: "projections", Short: "Manage projection scenarios and rules", Args: cli.UsageArgs(cobra.NoArgs), RunE: func(c *cobra.Command, _ []string) error { return c.Help() }}
	projections.AddCommand(m.scenarios(), m.rules(), m.calculate(), m.retirement())
	reports := &cobra.Command{Use: "reports", Short: "Inspect server-calculated portfolio reports", Args: cli.UsageArgs(cobra.NoArgs), RunE: func(c *cobra.Command, _ []string) error { return c.Help() }}
	reports.AddCommand(m.netWorth(), m.currencyExposure())
	rates := &cobra.Command{Use: "exchange-rates", Short: "List server-resolved exchange rates", Args: cli.UsageArgs(cobra.NoArgs)}
	rates.RunE = m.runRates(rates)
	root.AddCommand(projections, reports, rates)
}

func (m Module) scenarios() *cobra.Command {
	c := &cobra.Command{Use: "scenarios", Short: "Manage projection scenarios", Args: cli.UsageArgs(cobra.NoArgs)}
	c.AddCommand(m.scenarioList(), m.scenarioCreate(), m.scenarioUpdate(), m.scenarioDelete())
	return c
}
func portfolioFlag(c *cobra.Command, value *string) {
	c.Flags().StringVar(value, "portfolio", "", "portfolio ID")
	_ = c.MarkFlagRequired("portfolio")
}
func scenarioFlag(c *cobra.Command, value *string) {
	c.Flags().StringVar(value, "scenario", "", "scenario ID")
	_ = c.MarkFlagRequired("scenario")
}
func (m Module) scenarioList() *cobra.Command {
	var portfolio string
	c := &cobra.Command{Use: "list", Short: "List projection scenarios", Args: cli.UsageArgs(cobra.NoArgs), RunE: func(c *cobra.Command, _ []string) error {
		a, err := m.Factory(c.Context())
		if err != nil {
			return err
		}
		items, err := a.ScenarioList(c.Context(), portfolio)
		if err != nil {
			return err
		}
		return emitList(c, "scenarios", items, scenarioRows(c, items))
	}}
	portfolioFlag(c, &portfolio)
	return c
}

type scenarioFields struct {
	portfolio, name, color  string
	birthYear, displayOrder int
	active, clearBirthYear  bool
}

func addScenarioFields(c *cobra.Command, f *scenarioFields, create bool) {
	portfolioFlag(c, &f.portfolio)
	c.Flags().StringVar(&f.name, "name", "", "scenario name")
	c.Flags().StringVar(&f.color, "color", "", "display color")
	c.Flags().IntVar(&f.birthYear, "birth-year", 0, "birth year")
	c.Flags().IntVar(&f.displayOrder, "display-order", 0, "display order")
	c.Flags().BoolVar(&f.active, "active", false, "make scenario active")
	if create {
		_ = c.MarkFlagRequired("name")
	} else {
		c.Flags().BoolVar(&f.clearBirthYear, "clear-birth-year", false, "clear the birth year")
	}
}
func (m Module) scenarioCreate() *cobra.Command {
	var f scenarioFields
	c := &cobra.Command{Use: "create", Short: "Create a projection scenario", Args: cli.UsageArgs(cobra.NoArgs), RunE: func(c *cobra.Command, _ []string) error {
		if strings.TrimSpace(f.name) == "" {
			return usage("name must not be empty")
		}
		if f.birthYear != 0 && (f.birthYear < 1900 || f.birthYear > time.Now().Year()) {
			return usage("birth year is invalid")
		}
		body := daikuv1.ProjectionScenarioRequest{Name: f.name}
		if c.Flags().Changed("color") {
			body.Color = &f.color
		}
		if c.Flags().Changed("birth-year") {
			body.BirthYear = &f.birthYear
		}
		if c.Flags().Changed("display-order") {
			body.DisplayOrder = &f.displayOrder
		}
		if c.Flags().Changed("active") {
			body.IsActive = &f.active
		}
		a, err := m.Factory(c.Context())
		if err != nil {
			return err
		}
		result, err := a.ScenarioCreate(c.Context(), f.portfolio, body)
		return emit(c, result, "Projection scenario created.\n", err)
	}}
	addScenarioFields(c, &f, true)
	return c
}
func (m Module) scenarioUpdate() *cobra.Command {
	var f scenarioFields
	c := &cobra.Command{Use: "update <id>", Short: "Update a projection scenario", Args: cli.UsageArgs(cobra.ExactArgs(1)), RunE: func(c *cobra.Command, args []string) error {
		if !anyChanged(c, "name", "color", "birth-year", "clear-birth-year", "display-order", "active") {
			return usage("provide at least one field to update")
		}
		if c.Flags().Changed("birth-year") && f.clearBirthYear {
			return usage("birth-year and clear-birth-year cannot be used together")
		}
		if c.Flags().Changed("name") && strings.TrimSpace(f.name) == "" {
			return usage("name must not be empty")
		}
		if c.Flags().Changed("birth-year") && (f.birthYear < 1900 || f.birthYear > time.Now().Year()) {
			return usage("birth year is invalid")
		}
		body := scenarioPatch{}
		if c.Flags().Changed("name") {
			body.Name = &f.name
		}
		if c.Flags().Changed("color") {
			body.Color = &f.color
		}
		if c.Flags().Changed("birth-year") {
			value := &f.birthYear
			body.BirthYear = &value
		}
		if f.clearBirthYear {
			body.BirthYear = new(*int)
		}
		if c.Flags().Changed("display-order") {
			body.DisplayOrder = &f.displayOrder
		}
		if c.Flags().Changed("active") {
			body.IsActive = &f.active
		}
		a, err := m.Factory(c.Context())
		if err != nil {
			return err
		}
		result, err := a.ScenarioUpdate(c.Context(), f.portfolio, args[0], body)
		return emit(c, result, "Projection scenario updated.\n", err)
	}}
	addScenarioFields(c, &f, false)
	return c
}
func (m Module) scenarioDelete() *cobra.Command {
	var portfolio string
	var yes bool
	c := &cobra.Command{Use: "delete <id>", Short: "Delete a projection scenario", Args: cli.UsageArgs(cobra.ExactArgs(1)), RunE: func(c *cobra.Command, args []string) error {
		if !yes {
			message := cli.Human(c).Localizer.Humanf("Delete projection scenario %s.", args[0])
			if err := confirm(c, message); err != nil {
				return err
			}
		}
		a, err := m.Factory(c.Context())
		if err != nil {
			return err
		}
		err = a.ScenarioDelete(c.Context(), portfolio, args[0])
		return emit(c, map[string]any{"id": args[0], "deleted": err == nil}, "Projection scenario deleted.\n", err)
	}}
	portfolioFlag(c, &portfolio)
	c.Flags().BoolVar(&yes, "yes", false, "skip the interactive confirmation")
	return c
}

func (m Module) calculate() *cobra.Command {
	return m.resultCommand("calculate", "Calculate a projection on the server", func(a API, ctx context.Context, p, s string) (any, error) { return a.Calculate(ctx, p, s) })
}
func (m Module) retirement() *cobra.Command {
	return m.resultCommand("retirement", "Calculate retirement readiness on the server", func(a API, ctx context.Context, p, s string) (any, error) { return a.Retirement(ctx, p, s) })
}
func (m Module) resultCommand(use, short string, run func(API, context.Context, string, string) (any, error)) *cobra.Command {
	var p, s string
	c := &cobra.Command{Use: use, Short: short, Args: cli.UsageArgs(cobra.NoArgs), RunE: func(c *cobra.Command, _ []string) error {
		a, e := m.Factory(c.Context())
		if e != nil {
			return e
		}
		v, e := run(a, c.Context(), p, s)
		return emit(c, v, cli.Human(c).Localizer.Human(short)+".\n", e)
	}}
	portfolioFlag(c, &p)
	scenarioFlag(c, &s)
	return c
}

func (m Module) rules() *cobra.Command {
	c := &cobra.Command{Use: "rules", Short: "Manage projection rules", Args: cli.UsageArgs(cobra.NoArgs)}
	c.AddCommand(m.ruleList(), m.ruleCreate(), m.ruleUpdate(), m.ruleDelete())
	return c
}
func (m Module) ruleList() *cobra.Command {
	var s string
	c := &cobra.Command{Use: "list", Short: "List projection rules", Args: cli.UsageArgs(cobra.NoArgs), RunE: func(c *cobra.Command, _ []string) error {
		a, e := m.Factory(c.Context())
		if e != nil {
			return e
		}
		v, e := a.RuleList(c.Context(), s)
		if e != nil {
			return e
		}
		return emitList(c, "rules", v, ruleRows(c, v))
	}}
	scenarioFlag(c, &s)
	return c
}

type ruleFields struct {
	scenario, category, ruleType, config string
	enabled                              bool
	order                                int
}

func addRuleFields(c *cobra.Command, f *ruleFields, create bool) {
	scenarioFlag(c, &f.scenario)
	c.Flags().StringVar(&f.category, "category", "", "asset, debt, income or expense")
	c.Flags().StringVar(&f.ruleType, "type", "", "server rule type")
	c.Flags().StringVar(&f.config, "config", "", "JSON object with the server rule configuration")
	c.Flags().BoolVar(&f.enabled, "enabled", false, "enable rule")
	c.Flags().IntVar(&f.order, "display-order", 0, "display order")
	if create {
		for _, n := range []string{"category", "type", "config"} {
			_ = c.MarkFlagRequired(n)
		}
	}
}
func parseConfig(raw string) (daikuv1.ProjectionRuleConfigRequest, error) {
	var v daikuv1.ProjectionRuleConfigRequest
	if raw == "" {
		return v, usage("config is required")
	}
	decoder := json.NewDecoder(bytes.NewBufferString(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&v); err != nil {
		return v, usage("config must be a valid JSON object")
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return v, usage("config must contain exactly one JSON object")
	}
	var object map[string]json.RawMessage
	if json.Unmarshal([]byte(raw), &object) != nil || object == nil {
		return v, usage("config must be a valid JSON object")
	}
	if v.Currency != nil && !validConfigCurrency(string(*v.Currency)) {
		return v, usage("config currency is not allowed by the API contract")
	}
	if v.Frequency != nil && !validFrequency(string(*v.Frequency)) {
		return v, usage("config frequency is not allowed by the API contract")
	}
	if v.TargetBucketType != nil && !validTargetBucketType(string(*v.TargetBucketType)) {
		return v, usage("config target_bucket_type is not allowed by the API contract")
	}
	return v, nil
}
func validConfigCurrency(v string) bool { return v == "UYU" || v == "USD" || v == "EUR" }
func validFrequency(v string) bool {
	return v == "monthly" || v == "quarterly" || v == "semiannual" || v == "yearly"
}
func validTargetBucketType(v string) bool {
	return v == "cash" || v == "investments" || v == "crypto" || v == "real_estate" || v == "vehicles" || v == "other"
}
func validCategory(v string) bool {
	return v == "asset" || v == "debt" || v == "income" || v == "expense"
}
func (m Module) ruleCreate() *cobra.Command {
	var f ruleFields
	c := &cobra.Command{Use: "create", Short: "Create a projection rule", Args: cli.UsageArgs(cobra.NoArgs), RunE: func(c *cobra.Command, _ []string) error {
		if !validCategory(f.category) {
			return usage("category must be asset, debt, income or expense")
		}
		if strings.TrimSpace(f.ruleType) == "" {
			return usage("type must not be empty")
		}
		cfg, e := parseConfig(f.config)
		if e != nil {
			return e
		}
		body := daikuv1.ProjectionRuleRequest{Category: daikuv1.ProjectionRuleCategoryEnum(f.category), RuleType: f.ruleType, Config: cfg}
		if c.Flags().Changed("enabled") {
			body.IsEnabled = &f.enabled
		}
		if c.Flags().Changed("display-order") {
			body.DisplayOrder = &f.order
		}
		a, e := m.Factory(c.Context())
		if e != nil {
			return e
		}
		v, e := a.RuleCreate(c.Context(), f.scenario, body)
		return emit(c, v, "Projection rule created.\n", e)
	}}
	addRuleFields(c, &f, true)
	return c
}
func (m Module) ruleUpdate() *cobra.Command {
	var f ruleFields
	c := &cobra.Command{Use: "update <id>", Short: "Update a projection rule", Args: cli.UsageArgs(cobra.ExactArgs(1)), RunE: func(c *cobra.Command, args []string) error {
		if !anyChanged(c, "category", "type", "config", "enabled", "display-order") {
			return usage("provide at least one field to update")
		}
		body := daikuv1.PatchedProjectionRuleRequest{}
		if c.Flags().Changed("category") {
			if !validCategory(f.category) {
				return usage("category must be asset, debt, income or expense")
			}
			v := daikuv1.ProjectionRuleCategoryEnum(f.category)
			body.Category = &v
		}
		if c.Flags().Changed("type") {
			if strings.TrimSpace(f.ruleType) == "" {
				return usage("type must not be empty")
			}
			body.RuleType = &f.ruleType
		}
		if c.Flags().Changed("config") {
			v, e := parseConfig(f.config)
			if e != nil {
				return e
			}
			body.Config = &v
		}
		if c.Flags().Changed("enabled") {
			body.IsEnabled = &f.enabled
		}
		if c.Flags().Changed("display-order") {
			body.DisplayOrder = &f.order
		}
		a, e := m.Factory(c.Context())
		if e != nil {
			return e
		}
		v, e := a.RuleUpdate(c.Context(), f.scenario, args[0], body)
		return emit(c, v, "Projection rule updated.\n", e)
	}}
	addRuleFields(c, &f, false)
	return c
}
func (m Module) ruleDelete() *cobra.Command {
	var s string
	var yes bool
	c := &cobra.Command{Use: "delete <id>", Short: "Delete a projection rule", Args: cli.UsageArgs(cobra.ExactArgs(1)), RunE: func(c *cobra.Command, args []string) error {
		if !yes {
			message := cli.Human(c).Localizer.Humanf("Delete projection rule %s.", args[0])
			if e := confirm(c, message); e != nil {
				return e
			}
		}
		a, e := m.Factory(c.Context())
		if e != nil {
			return e
		}
		e = a.RuleDelete(c.Context(), s, args[0])
		return emit(c, map[string]any{"id": args[0], "deleted": e == nil}, "Projection rule deleted.\n", e)
	}}
	scenarioFlag(c, &s)
	c.Flags().BoolVar(&yes, "yes", false, "skip the interactive confirmation")
	return c
}

func (m Module) netWorth() *cobra.Command {
	return reportCommand(m, "net-worth", "Show the server-calculated net worth series", func(a API, c *cobra.Command) (any, []output.Row, error) {
		v, e := a.NetWorth(c.Context())
		if e != nil {
			return nil, nil, e
		}
		rows := make([]output.Row, 0, len(v.Series))
		for _, p := range v.Series {
			rows = append(rows, output.Row{{Label: label(c, "DATE"), Value: p.Date.String()}, {Label: label(c, "NET WORTH"), Value: p.NetWorth}, {Label: label(c, "ASSETS"), Value: p.Assets}, {Label: label(c, "LIABILITIES"), Value: p.Liabilities}, {Label: label(c, "CURRENCY"), Value: string(v.Currency)}})
		}
		return v, rows, nil
	})
}
func (m Module) currencyExposure() *cobra.Command {
	return reportCommand(m, "currency-exposure", "Show server-calculated currency exposure", func(a API, c *cobra.Command) (any, []output.Row, error) {
		v, e := a.CurrencyExposure(c.Context())
		if e != nil {
			return nil, nil, e
		}
		rows := make([]output.Row, 0, len(v.ByCurrency))
		for _, p := range v.ByCurrency {
			rows = append(rows, output.Row{{Label: label(c, "CURRENCY"), Value: string(p.Currency)}, {Label: label(c, "NATIVE"), Value: p.NativeTotal}, {Label: label(c, "CONVERTED"), Value: p.ConvertedTotal}, {Label: label(c, "PERCENT"), Value: p.Pct}, {Label: label(c, "AS OF"), Value: v.AsOf.String()}})
		}
		return v, rows, nil
	})
}
func reportCommand(m Module, use, short string, run func(API, *cobra.Command) (any, []output.Row, error)) *cobra.Command {
	return &cobra.Command{Use: use, Short: short, Args: cli.UsageArgs(cobra.NoArgs), RunE: func(c *cobra.Command, _ []string) error {
		a, e := m.Factory(c.Context())
		if e != nil {
			return e
		}
		v, rows, e := run(a, c)
		if e != nil {
			return e
		}
		if jsonMode(c) {
			return cli.WriteSuccess(c.OutOrStdout(), v)
		}
		return renderer(c).Table(rows)
	}}
}
func (m Module) runRates(c *cobra.Command) func(*cobra.Command, []string) error {
	var date string
	c.Flags().StringVar(&date, "date", "", "requested date (YYYY-MM-DD; server resolves prior business day)")
	return func(c *cobra.Command, _ []string) error {
		var params daikuv1.DaikuExchangeRatesGetParams
		if date != "" {
			d, e := time.Parse("2006-01-02", date)
			if e != nil {
				return usage("date must use YYYY-MM-DD")
			}
			v := types.Date{Time: d}
			params.Date = &v
		}
		a, e := m.Factory(c.Context())
		if e != nil {
			return e
		}
		rates, e := a.Rates(c.Context(), &params)
		if e != nil {
			return e
		}
		rows := make([]output.Row, 0, len(rates))
		for _, r := range rates {
			resolved := ""
			if r.Date != nil {
				resolved = r.Date.String()
			}
			rows = append(rows, output.Row{{Label: label(c, "DATE"), Value: resolved}, {Label: label(c, "FROM"), Value: string(r.FromCurrency)}, {Label: label(c, "TO"), Value: string(r.ToCurrency)}, {Label: label(c, "RATE"), Value: r.Rate}})
		}
		return emitList(c, "rates", rates, rows)
	}
}

func scenarioRows(c *cobra.Command, v []daikuv1.ProjectionScenario) []output.Row {
	rows := make([]output.Row, 0, len(v))
	for _, x := range v {
		id := ""
		if x.Id != nil {
			id = *x.Id
		}
		rows = append(rows, output.Row{{Label: label(c, "ID"), Value: id}, {Label: label(c, "NAME"), Value: x.Name}})
	}
	return rows
}
func ruleRows(c *cobra.Command, v []daikuv1.ProjectionRule) []output.Row {
	rows := make([]output.Row, 0, len(v))
	for _, x := range v {
		id := ""
		if x.Id != nil {
			id = *x.Id
		}
		rows = append(rows, output.Row{{Label: label(c, "ID"), Value: id}, {Label: label(c, "CATEGORY"), Value: string(x.Category)}, {Label: label(c, "TYPE"), Value: x.RuleType}})
	}
	return rows
}
func emitList(c *cobra.Command, key string, value any, rows []output.Row) error {
	if jsonMode(c) {
		return cli.WriteSuccess(c.OutOrStdout(), map[string]any{key: value})
	}
	return renderer(c).Table(rows)
}
func renderer(c *cobra.Command) output.Renderer {
	h := cli.Human(c)
	return output.Renderer{Writer: c.OutOrStdout(), Localize: h.Localizer, Terminal: h.Terminal, Width: h.Width, NoColor: h.NoColor}
}
func label(c *cobra.Command, value string) string { return cli.Human(c).Localizer.Human(value) }
func emit(c *cobra.Command, data any, message string, err error) error {
	if err != nil {
		return err
	}
	if jsonMode(c) {
		return cli.WriteSuccess(c.OutOrStdout(), data)
	}
	_, err = fmt.Fprint(c.OutOrStdout(), cli.Human(c).Localizer.Human(message))
	return err
}
func jsonMode(c *cobra.Command) bool { v, _ := c.Flags().GetBool("json"); return v }
func anyChanged(c *cobra.Command, names ...string) bool {
	for _, n := range names {
		if c.Flags().Changed(n) {
			return true
		}
	}
	return false
}
func usage(s string) *cli.Error {
	return &cli.Error{Code: "usage_error", Message: s, ExitCode: cli.ExitUsage}
}
func apiFailure() *cli.Error {
	return &cli.Error{Code: "api_error", Message: "the Daiku API request failed", ExitCode: cli.ExitFailure}
}
func status(code int) error {
	switch {
	case code >= 200 && code < 300:
		return nil
	case code == 400:
		return usage("the request was rejected by the server")
	case code == 401:
		return &cli.Error{Code: "unauthorized", Message: "authentication is required", ExitCode: cli.ExitAuth}
	case code == 403:
		return &cli.Error{Code: "forbidden", Message: "your role does not allow this operation", ExitCode: cli.ExitForbidden}
	case code == 404:
		return &cli.Error{Code: "not_found", Message: "the requested resource was not found", ExitCode: cli.ExitNotFound}
	default:
		return apiFailure()
	}
}
func confirm(c *cobra.Command, message string) error {
	h := cli.Human(c)
	p := prompt.Prompter{In: c.InOrStdin(), Out: c.ErrOrStderr(), Localize: h.Localizer, Terminal: h.Interactive && !h.JSON}
	if e := p.ConfirmDestructive(h.Localizer.Human(message)); e != nil {
		if errors.Is(e, prompt.ErrNonInteractive) {
			return &cli.Error{Code: "confirmation_required", Message: "confirmation requires an interactive terminal; pass --yes to continue", ExitCode: cli.ExitUsage}
		}
		if errors.Is(e, prompt.ErrAborted) {
			return &cli.Error{Code: "operation_cancelled", Message: "operation cancelled", ExitCode: cli.ExitConflict}
		}
		return apiFailure()
	}
	return nil
}

type generatedAPI struct{ c *daikuv1.ClientWithResponses }

func (a generatedAPI) ScenarioList(c context.Context, p string) ([]daikuv1.ProjectionScenario, error) {
	r, e := a.c.DaikuPortfoliosPortfolioPkScenariosGetWithResponse(c, p)
	if e != nil {
		return nil, apiFailure()
	}
	if e = status(r.StatusCode()); e != nil {
		return nil, e
	}
	return *r.JSON200, nil
}
func (a generatedAPI) ScenarioCreate(c context.Context, p string, b daikuv1.ProjectionScenarioRequest) (*daikuv1.ProjectionScenario, error) {
	r, e := a.c.DaikuPortfoliosPortfolioPkScenariosPostWithResponse(c, p, b)
	if e != nil {
		return nil, apiFailure()
	}
	if e = status(r.StatusCode()); e != nil {
		return nil, e
	}
	return r.JSON201, nil
}
func (a generatedAPI) ScenarioUpdate(c context.Context, p, id string, b scenarioPatch) (*daikuv1.ProjectionScenario, error) {
	payload, e := json.Marshal(b)
	if e != nil {
		return nil, apiFailure()
	}
	r, e := a.c.DaikuPortfoliosPortfolioPkScenariosIdPatchWithBodyWithResponse(c, p, id, "application/json", bytes.NewReader(payload))
	if e != nil {
		return nil, apiFailure()
	}
	if e = status(r.StatusCode()); e != nil {
		return nil, e
	}
	return r.JSON200, nil
}
func (a generatedAPI) ScenarioDelete(c context.Context, p, id string) error {
	r, e := a.c.DaikuPortfoliosPortfolioPkScenariosIdDeleteWithResponse(c, p, id)
	if e != nil {
		return apiFailure()
	}
	return status(r.StatusCode())
}
func (a generatedAPI) Calculate(c context.Context, p, id string) (*daikuv1.ProjectionResult, error) {
	r, e := a.c.DaikuPortfoliosPortfolioPkScenariosIdCalculateGetWithResponse(c, p, id)
	if e != nil {
		return nil, apiFailure()
	}
	if e = status(r.StatusCode()); e != nil {
		return nil, e
	}
	return r.JSON200, nil
}
func (a generatedAPI) Retirement(c context.Context, p, id string) (*daikuv1.RetirementResult, error) {
	r, e := a.c.DaikuPortfoliosPortfolioPkScenariosIdRetirementGetWithResponse(c, p, id)
	if e != nil {
		return nil, apiFailure()
	}
	if e = status(r.StatusCode()); e != nil {
		return nil, e
	}
	return r.JSON200, nil
}
func (a generatedAPI) RuleList(c context.Context, s string) ([]daikuv1.ProjectionRule, error) {
	r, e := a.c.DaikuScenariosScenarioPkRulesGetWithResponse(c, s)
	if e != nil {
		return nil, apiFailure()
	}
	if e = status(r.StatusCode()); e != nil {
		return nil, e
	}
	return *r.JSON200, nil
}
func (a generatedAPI) RuleCreate(c context.Context, s string, b daikuv1.ProjectionRuleRequest) (*daikuv1.ProjectionRule, error) {
	r, e := a.c.DaikuScenariosScenarioPkRulesPostWithResponse(c, s, b)
	if e != nil {
		return nil, apiFailure()
	}
	if e = status(r.StatusCode()); e != nil {
		return nil, e
	}
	return r.JSON201, nil
}
func (a generatedAPI) RuleUpdate(c context.Context, s, id string, b daikuv1.PatchedProjectionRuleRequest) (*daikuv1.ProjectionRule, error) {
	r, e := a.c.DaikuScenariosScenarioPkRulesIdPatchWithResponse(c, s, id, b)
	if e != nil {
		return nil, apiFailure()
	}
	if e = status(r.StatusCode()); e != nil {
		return nil, e
	}
	return r.JSON200, nil
}
func (a generatedAPI) RuleDelete(c context.Context, s, id string) error {
	r, e := a.c.DaikuScenariosScenarioPkRulesIdDeleteWithResponse(c, s, id)
	if e != nil {
		return apiFailure()
	}
	return status(r.StatusCode())
}
func (a generatedAPI) NetWorth(c context.Context) (*daikuv1.NetWorthSeries, error) {
	r, e := a.c.DaikuPortfoliosReportsNetWorthGetWithResponse(c)
	if e != nil {
		return nil, apiFailure()
	}
	if e = status(r.StatusCode()); e != nil {
		return nil, e
	}
	return r.JSON200, nil
}
func (a generatedAPI) CurrencyExposure(c context.Context) (*daikuv1.CurrencyExposure, error) {
	r, e := a.c.DaikuPortfoliosReportsCurrencyExposureGetWithResponse(c)
	if e != nil {
		return nil, apiFailure()
	}
	if e = status(r.StatusCode()); e != nil {
		return nil, e
	}
	return r.JSON200, nil
}
func (a generatedAPI) Rates(c context.Context, p *daikuv1.DaikuExchangeRatesGetParams) ([]daikuv1.ExchangeRate, error) {
	r, e := a.c.DaikuExchangeRatesGetWithResponse(c, p)
	if e != nil {
		return nil, apiFailure()
	}
	if e = status(r.StatusCode()); e != nil {
		return nil, e
	}
	return *r.JSON200, nil
}
