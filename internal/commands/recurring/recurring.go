package recurring

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	openapi_types "github.com/oapi-codegen/runtime/types"
	"github.com/spf13/cobra"

	daikuv1 "github.com/DaikuFi/daiku-cli/generated/daikuv1"
	"github.com/DaikuFi/daiku-cli/internal/agent"
	authcore "github.com/DaikuFi/daiku-cli/internal/auth"
	"github.com/DaikuFi/daiku-cli/internal/cli"
	"github.com/DaikuFi/daiku-cli/internal/currency"
	"github.com/DaikuFi/daiku-cli/internal/profiles"
	"github.com/DaikuFi/daiku-cli/internal/prompt"
)

type API interface {
	List(context.Context, string) ([]daikuv1.RecurringExpense, error)
	Create(context.Context, string, daikuv1.RecurringExpenseRequest) (*daikuv1.RecurringExpense, error)
	Update(context.Context, string, string, Patch) (*daikuv1.RecurringExpense, error)
	Delete(context.Context, string, string) error
	Occurrences(context.Context, string, *daikuv1.DaikuHouseholdsHouseholdPkRecurringOccurrencesGetParams) ([]daikuv1.RecurringOccurrence, error)
	Confirm(context.Context, string, string, daikuv1.RecurringOccurrenceConfirmRequestRequest) (*daikuv1.RecurringOccurrence, error)
	Skip(context.Context, string, string) (*daikuv1.RecurringOccurrence, error)
	Snooze(context.Context, string, string, daikuv1.RecurringOccurrenceSnoozeRequestRequest) (*daikuv1.RecurringOccurrence, error)
}
type Factory func(context.Context) (API, error)
type Module struct{ Factory Factory }
type Patch map[string]any

func New(store profiles.Store, manager *authcore.Manager) Module {
	return Module{Factory: func(ctx context.Context) (API, error) {
		cfg, err := store.Load()
		if err != nil || cfg.Current == "" {
			return nil, authError()
		}
		profile := cfg.Profiles[cfg.Current]
		token, err := manager.AccessToken(ctx, cfg.Current)
		if err != nil {
			return nil, authError()
		}
		serverURL := strings.TrimSuffix(profile.APIURL, "api/v1/")
		client, err := daikuv1.NewClientWithResponses(serverURL, daikuv1.WithRequestEditorFn(func(_ context.Context, req *http.Request) error {
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
	cmd := &cobra.Command{Use: "recurring", Short: "Manage recurring templates and their occurrences", Args: cli.UsageArgs(cobra.NoArgs), RunE: func(cmd *cobra.Command, _ []string) error { return cmd.Help() }}
	cmd.AddCommand(agent.ReadOnly(m.list()), m.create(), m.update(), m.delete(), m.occurrences())
	root.AddCommand(cmd)
}
func householdFlag(cmd *cobra.Command, value *string) {
	cmd.Flags().StringVar(value, "household", "", "household ID")
	_ = cmd.MarkFlagRequired("household")
}

func (m Module) list() *cobra.Command {
	var household string
	cmd := &cobra.Command{Use: "list", Short: "List recurring templates", Args: cli.UsageArgs(cobra.NoArgs), RunE: func(cmd *cobra.Command, _ []string) error {
		a, e := m.Factory(cmd.Context())
		if e != nil {
			return e
		}
		v, e := a.List(cmd.Context(), household)
		return emit(cmd, map[string]any{"templates": v}, "%d recurring templates.\n", len(v), e)
	}}
	householdFlag(cmd, &household)
	return cmd
}

type templateFlags struct {
	household, description, amount, currency, frequency, transactionType, creationMode, account, destinationAccount, category, startDate string
	day, month                                                                                                                           int
	active                                                                                                                               bool
	clearAccount, clearDestinationAccount, clearCategory, clearMonth                                                                     bool
}

func addTemplateFlags(cmd *cobra.Command, f *templateFlags, create bool) {
	householdFlag(cmd, &f.household)
	cmd.Flags().StringVar(&f.description, "description", "", "description")
	cmd.Flags().StringVar(&f.amount, "amount", "", "amount")
	cmd.Flags().StringVar(&f.currency, "currency", "", "ISO currency published by the API contract")
	cmd.Flags().StringVar(&f.frequency, "frequency", "", "frequency: monthly or yearly")
	cmd.Flags().StringVar(&f.transactionType, "type", "", "transaction type: expense, income or transfer")
	cmd.Flags().StringVar(&f.creationMode, "creation-mode", "", "creation mode: auto or confirm")
	cmd.Flags().StringVar(&f.account, "account", "", "source account ID")
	cmd.Flags().StringVar(&f.destinationAccount, "destination-account", "", "destination account ID for transfers")
	cmd.Flags().StringVar(&f.category, "category", "", "category ID")
	cmd.Flags().StringVar(&f.startDate, "start-date", "", "start date (YYYY-MM-DD)")
	cmd.Flags().IntVar(&f.day, "day", 0, "day of month (1-31)")
	cmd.Flags().IntVar(&f.month, "month", 0, "month of year for yearly templates (1-12)")
	cmd.Flags().BoolVar(&f.active, "active", true, "whether the template is active")
	if create {
		for _, n := range []string{"description", "amount", "currency", "frequency", "type", "creation-mode", "day"} {
			_ = cmd.MarkFlagRequired(n)
		}
	} else {
		cmd.Flags().BoolVar(&f.clearAccount, "clear-account", false, "clear the source account")
		cmd.Flags().BoolVar(&f.clearDestinationAccount, "clear-destination-account", false, "clear the destination account")
		cmd.Flags().BoolVar(&f.clearCategory, "clear-category", false, "clear the category")
		cmd.Flags().BoolVar(&f.clearMonth, "clear-month", false, "clear the month of year")
	}
}
func validateTemplate(f templateFlags, partial bool) error {
	if !partial && (f.description == "" || f.amount == "" || f.currency == "" || f.frequency == "" || f.transactionType == "" || f.creationMode == "" || f.day == 0) {
		return usage("description, amount, currency, frequency, type, creation-mode and day are required")
	}
	if f.currency != "" && !validCurrency(f.currency) {
		return usage("currency is not supported by the Daiku API contract")
	}
	if f.frequency != "" && f.frequency != "monthly" && f.frequency != "yearly" {
		return usage("frequency must be monthly or yearly")
	}
	if f.transactionType != "" && f.transactionType != "expense" && f.transactionType != "income" && f.transactionType != "transfer" {
		return usage("type must be expense, income or transfer")
	}
	if f.creationMode != "" && f.creationMode != "auto" && f.creationMode != "confirm" {
		return usage("creation-mode must be auto or confirm")
	}
	if f.day < 0 || f.day > 31 {
		return usage("day must be between 1 and 31")
	}
	if f.month < 0 || f.month > 12 {
		return usage("month must be between 1 and 12")
	}
	if f.frequency == "yearly" && f.month == 0 {
		return usage("yearly frequency requires --month")
	}
	if f.frequency == "monthly" && f.month != 0 {
		return usage("month is only valid for yearly frequency")
	}
	if f.transactionType == "transfer" && (f.account == "" || f.destinationAccount == "") {
		return usage("transfer templates require --account and --destination-account")
	}
	if f.transactionType != "" && f.transactionType != "transfer" && f.destinationAccount != "" {
		return usage("destination-account is only valid for transfers")
	}
	if f.startDate != "" {
		if _, e := parseDate(f.startDate); e != nil {
			return e
		}
	}
	if (f.account != "" && f.clearAccount) || (f.destinationAccount != "" && f.clearDestinationAccount) || (f.category != "" && f.clearCategory) || (f.month != 0 && f.clearMonth) {
		return usage("a field cannot be set and cleared together")
	}
	return nil
}
func validCurrency(value string) bool {
	return currency.IsSupported(value)
}
func request(f templateFlags) (daikuv1.RecurringExpenseRequest, error) {
	r := daikuv1.RecurringExpenseRequest{Description: f.description, Amount: f.amount, DayOfMonth: f.day, Frequency: daikuv1.FrequencyDceEnum(f.frequency)}
	currency := daikuv1.Currency3e8Enum(f.currency)
	kind := daikuv1.RecurringExpenseTransactionTypeEnum(f.transactionType)
	mode := daikuv1.CreationModeEnum(f.creationMode)
	r.Currency = &currency
	r.TransactionType = &kind
	r.CreationMode = &mode
	r.IsActive = &f.active
	if f.account != "" {
		r.Account = &f.account
	}
	if f.destinationAccount != "" {
		r.DestinationAccount = &f.destinationAccount
	}
	if f.category != "" {
		r.Category = &f.category
	}
	if f.month != 0 {
		r.MonthOfYear = &f.month
	}
	if f.startDate != "" {
		d, e := parseDate(f.startDate)
		if e != nil {
			return r, e
		}
		r.StartDate = &d
	}
	return r, nil
}
func (m Module) create() *cobra.Command {
	var f templateFlags
	cmd := &cobra.Command{Use: "create", Short: "Create a recurring template", Args: cli.UsageArgs(cobra.NoArgs), RunE: func(cmd *cobra.Command, _ []string) error {
		if e := validateTemplate(f, false); e != nil {
			return e
		}
		body, e := request(f)
		if e != nil {
			return e
		}
		a, e := m.Factory(cmd.Context())
		if e != nil {
			return e
		}
		v, e := a.Create(cmd.Context(), f.household, body)
		return emit(cmd, v, "Recurring template created.\n", e)
	}}
	addTemplateFlags(cmd, &f, true)
	return cmd
}
func (m Module) update() *cobra.Command {
	var f templateFlags
	cmd := &cobra.Command{Use: "update <id>", Short: "Update a recurring template", Args: cli.UsageArgs(cobra.ExactArgs(1)), RunE: func(cmd *cobra.Command, args []string) error {
		if e := validateTemplate(f, true); e != nil {
			return e
		}
		if cmd.Flags().Changed("day") && (f.day < 1 || f.day > 31) {
			return usage("day must be between 1 and 31")
		}
		if cmd.Flags().Changed("month") && (f.month < 1 || f.month > 12) {
			return usage("month must be between 1 and 12")
		}
		names := []string{"description", "amount", "currency", "frequency", "type", "creation-mode", "account", "destination-account", "category", "start-date", "day", "month", "active", "clear-account", "clear-destination-account", "clear-category", "clear-month"}
		changed := false
		for _, n := range names {
			changed = changed || cmd.Flags().Changed(n)
		}
		if !changed {
			return usage("provide at least one field to update")
		}
		b := Patch{}
		if cmd.Flags().Changed("description") {
			b["description"] = f.description
		}
		if cmd.Flags().Changed("amount") {
			b["amount"] = f.amount
		}
		if cmd.Flags().Changed("currency") {
			b["currency"] = f.currency
		}
		if cmd.Flags().Changed("frequency") {
			b["frequency"] = f.frequency
		}
		if cmd.Flags().Changed("type") {
			b["transaction_type"] = f.transactionType
		}
		if cmd.Flags().Changed("creation-mode") {
			b["creation_mode"] = f.creationMode
		}
		if cmd.Flags().Changed("account") {
			b["account"] = f.account
		}
		if cmd.Flags().Changed("destination-account") {
			b["destination_account"] = f.destinationAccount
		}
		if cmd.Flags().Changed("category") {
			b["category"] = f.category
		}
		if cmd.Flags().Changed("day") {
			b["day_of_month"] = f.day
		}
		if cmd.Flags().Changed("month") {
			b["month_of_year"] = f.month
		}
		if cmd.Flags().Changed("active") {
			b["is_active"] = f.active
		}
		if cmd.Flags().Changed("start-date") {
			d, e := parseDate(f.startDate)
			if e != nil {
				return e
			}
			b["start_date"] = d
		}
		if f.clearAccount {
			b["account"] = nil
		}
		if f.clearDestinationAccount {
			b["destination_account"] = nil
		}
		if f.clearCategory {
			b["category"] = nil
		}
		if f.clearMonth {
			b["month_of_year"] = nil
		}
		a, e := m.Factory(cmd.Context())
		if e != nil {
			return e
		}
		v, e := a.Update(cmd.Context(), f.household, args[0], b)
		return emit(cmd, v, "Recurring template updated.\n", e)
	}}
	addTemplateFlags(cmd, &f, false)
	return cmd
}
func (m Module) delete() *cobra.Command {
	var h string
	var yes bool
	cmd := &cobra.Command{Use: "delete <id>", Short: "Delete a recurring template", Args: cli.UsageArgs(cobra.ExactArgs(1)), RunE: func(cmd *cobra.Command, args []string) error {
		if !yes {
			human := cli.Human(cmd)
			p := prompt.Prompter{In: cmd.InOrStdin(), Out: cmd.ErrOrStderr(), Localize: human.Localizer, Terminal: human.Interactive && !human.JSON}
			if e := p.ConfirmDestructive(human.Localizer.Humanf("Delete recurring template %s.", args[0])); e != nil {
				return confirmationError(e)
			}
		}
		a, e := m.Factory(cmd.Context())
		if e != nil {
			return e
		}
		e = a.Delete(cmd.Context(), h, args[0])
		return emit(cmd, map[string]any{"id": args[0], "deleted": e == nil}, "Recurring template deleted.\n", e)
	}}
	householdFlag(cmd, &h)
	cmd.Flags().BoolVar(&yes, "yes", false, "skip the interactive confirmation")
	return cmd
}

func (m Module) occurrences() *cobra.Command {
	cmd := &cobra.Command{Use: "occurrences", Short: "List and resolve server-generated occurrences", Args: cli.UsageArgs(cobra.NoArgs)}
	cmd.AddCommand(agent.ReadOnly(m.occurrenceList()), m.confirm(), m.skip(), m.snooze())
	return cmd
}
func (m Module) occurrenceList() *cobra.Command {
	var h, statusValue string
	cmd := &cobra.Command{Use: "list", Short: "List recurring occurrences", Args: cli.UsageArgs(cobra.NoArgs), RunE: func(cmd *cobra.Command, _ []string) error {
		if statusValue != "all" && statusValue != "pending" && statusValue != "confirmed" && statusValue != "skipped" && statusValue != "superseded" {
			return usage("status must be all, pending, confirmed, skipped or superseded")
		}
		s := daikuv1.DaikuHouseholdsHouseholdPkRecurringOccurrencesGetParamsStatus(statusValue)
		a, e := m.Factory(cmd.Context())
		if e != nil {
			return e
		}
		v, e := a.Occurrences(cmd.Context(), h, &daikuv1.DaikuHouseholdsHouseholdPkRecurringOccurrencesGetParams{Status: &s})
		return emit(cmd, map[string]any{"occurrences": v}, "%d recurring occurrences.\n", len(v), e)
	}}
	householdFlag(cmd, &h)
	cmd.Flags().StringVar(&statusValue, "status", "pending", "occurrence status")
	return cmd
}
func (m Module) confirm() *cobra.Command {
	var h, date, amount, toAmount string
	var yes bool
	cmd := &cobra.Command{Use: "confirm <id>", Short: "Confirm an occurrence as a human entry", Args: cli.UsageArgs(cobra.ExactArgs(1)), RunE: func(cmd *cobra.Command, args []string) error {
		d, e := parseDate(date)
		if e != nil {
			return e
		}
		if amount == "" {
			return usage("amount is required")
		}
		if !yes {
			human := cli.Human(cmd)
			p := prompt.Prompter{In: cmd.InOrStdin(), Out: cmd.ErrOrStderr(), Localize: human.Localizer, Terminal: human.Interactive && !human.JSON}
			if e = p.ConfirmDestructive(human.Localizer.Humanf("Confirm recurring occurrence %s.", args[0])); e != nil {
				return confirmationError(e)
			}
		}
		body := daikuv1.RecurringOccurrenceConfirmRequestRequest{FinalAmount: amount, FinalDate: d}
		if toAmount != "" {
			body.FinalToAmount = &toAmount
		}
		a, e := m.Factory(cmd.Context())
		if e != nil {
			return e
		}
		v, e := a.Confirm(cmd.Context(), h, args[0], body)
		return emit(cmd, v, "Recurring occurrence confirmed.\n", e)
	}}
	householdFlag(cmd, &h)
	cmd.Flags().StringVar(&date, "date", "", "final date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&amount, "amount", "", "final amount")
	cmd.Flags().StringVar(&toAmount, "to-amount", "", "final destination amount for transfers")
	cmd.Flags().BoolVar(&yes, "yes", false, "skip the interactive confirmation")
	_ = cmd.MarkFlagRequired("date")
	_ = cmd.MarkFlagRequired("amount")
	return cmd
}
func (m Module) skip() *cobra.Command {
	var h string
	var yes bool
	cmd := &cobra.Command{Use: "skip <id>", Short: "Skip a pending occurrence", Args: cli.UsageArgs(cobra.ExactArgs(1)), RunE: func(cmd *cobra.Command, args []string) error {
		if !yes {
			human := cli.Human(cmd)
			p := prompt.Prompter{In: cmd.InOrStdin(), Out: cmd.ErrOrStderr(), Localize: human.Localizer, Terminal: human.Interactive && !human.JSON}
			if e := p.ConfirmDestructive(human.Localizer.Humanf("Skip recurring occurrence %s.", args[0])); e != nil {
				return confirmationError(e)
			}
		}
		a, e := m.Factory(cmd.Context())
		if e != nil {
			return e
		}
		v, e := a.Skip(cmd.Context(), h, args[0])
		return emit(cmd, v, "Recurring occurrence skipped.\n", e)
	}}
	householdFlag(cmd, &h)
	cmd.Flags().BoolVar(&yes, "yes", false, "skip the interactive confirmation")
	return cmd
}
func (m Module) snooze() *cobra.Command {
	var h, until string
	cmd := &cobra.Command{Use: "snooze <id>", Short: "Snooze a pending occurrence", Args: cli.UsageArgs(cobra.ExactArgs(1)), RunE: func(cmd *cobra.Command, args []string) error {
		d, e := parseDate(until)
		if e != nil {
			return e
		}
		a, e := m.Factory(cmd.Context())
		if e != nil {
			return e
		}
		v, e := a.Snooze(cmd.Context(), h, args[0], daikuv1.RecurringOccurrenceSnoozeRequestRequest{Until: d})
		return emit(cmd, v, "Recurring occurrence snoozed.\n", e)
	}}
	householdFlag(cmd, &h)
	cmd.Flags().StringVar(&until, "until", "", "reminder date (YYYY-MM-DD)")
	_ = cmd.MarkFlagRequired("until")
	return cmd
}

func parseDate(value string) (openapi_types.Date, error) {
	parsed, e := time.Parse("2006-01-02", value)
	if e != nil {
		return openapi_types.Date{}, usage("date must use YYYY-MM-DD")
	}
	return openapi_types.Date{Time: parsed}, nil
}
func emit(cmd *cobra.Command, data any, format string, args ...any) error {
	last := args[len(args)-1]
	if e, _ := last.(error); e != nil {
		return e
	}
	args = args[:len(args)-1]
	jsonOut, _ := cmd.Flags().GetBool("json")
	if jsonOut {
		return cli.WriteSuccess(cmd.OutOrStdout(), data)
	}
	_, e := fmt.Fprint(cmd.OutOrStdout(), cli.Human(cmd).Localizer.Humanf(format, args...))
	return e
}
func usage(message string) *cli.Error {
	return &cli.Error{Code: "usage_error", Message: message, ExitCode: cli.ExitUsage}
}
func authError() *cli.Error {
	return &cli.Error{Code: "authentication_required", Message: "select and authenticate a profile first", ExitCode: cli.ExitAuth}
}
func apiFailure() *cli.Error {
	return &cli.Error{Code: "api_error", Message: "the Daiku API request failed", ExitCode: cli.ExitFailure}
}
func confirmationError(e error) *cli.Error {
	if errors.Is(e, prompt.ErrNonInteractive) {
		return &cli.Error{Code: "confirmation_required", Message: "confirmation requires an interactive terminal; pass --yes to continue", ExitCode: cli.ExitUsage}
	}
	if errors.Is(e, prompt.ErrAborted) {
		return &cli.Error{Code: "operation_cancelled", Message: "operation cancelled", ExitCode: cli.ExitConflict}
	}
	return apiFailure()
}
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
	if code == 409 {
		return &cli.Error{Code: "conflict", Message: "the occurrence is already resolved", ExitCode: cli.ExitConflict}
	}
	return apiFailure()
}

type generatedAPI struct{ c *daikuv1.ClientWithResponses }

func (a generatedAPI) List(ctx context.Context, h string) ([]daikuv1.RecurringExpense, error) {
	r, e := a.c.DaikuHouseholdsHouseholdPkRecurringGetWithResponse(ctx, h)
	if e != nil {
		return nil, apiFailure()
	}
	if e = status(r.StatusCode()); e != nil {
		return nil, e
	}
	return *r.JSON200, nil
}
func (a generatedAPI) Create(ctx context.Context, h string, b daikuv1.RecurringExpenseRequest) (*daikuv1.RecurringExpense, error) {
	r, e := a.c.DaikuHouseholdsHouseholdPkRecurringPostWithResponse(ctx, h, b)
	if e != nil {
		return nil, apiFailure()
	}
	if e = status(r.StatusCode()); e != nil {
		return nil, e
	}
	return r.JSON201, nil
}
func (a generatedAPI) Update(ctx context.Context, h, id string, b Patch) (*daikuv1.RecurringExpense, error) {
	payload, e := json.Marshal(b)
	if e != nil {
		return nil, apiFailure()
	}
	r, e := a.c.DaikuHouseholdsHouseholdPkRecurringIdPatchWithBodyWithResponse(ctx, h, id, "application/json", bytes.NewReader(payload))
	if e != nil {
		return nil, apiFailure()
	}
	if e = status(r.StatusCode()); e != nil {
		return nil, e
	}
	return r.JSON200, nil
}
func (a generatedAPI) Delete(ctx context.Context, h, id string) error {
	r, e := a.c.DaikuHouseholdsHouseholdPkRecurringIdDeleteWithResponse(ctx, h, id)
	if e != nil {
		return apiFailure()
	}
	return status(r.StatusCode())
}
func (a generatedAPI) Occurrences(ctx context.Context, h string, p *daikuv1.DaikuHouseholdsHouseholdPkRecurringOccurrencesGetParams) ([]daikuv1.RecurringOccurrence, error) {
	r, e := a.c.DaikuHouseholdsHouseholdPkRecurringOccurrencesGetWithResponse(ctx, h, p)
	if e != nil {
		return nil, apiFailure()
	}
	if e = status(r.StatusCode()); e != nil {
		return nil, e
	}
	return *r.JSON200, nil
}
func (a generatedAPI) Confirm(ctx context.Context, h, id string, b daikuv1.RecurringOccurrenceConfirmRequestRequest) (*daikuv1.RecurringOccurrence, error) {
	r, e := a.c.DaikuHouseholdsHouseholdPkRecurringOccurrencesIdConfirmPostWithResponse(ctx, h, id, b)
	if e != nil {
		return nil, apiFailure()
	}
	if e = status(r.StatusCode()); e != nil {
		return nil, e
	}
	return r.JSON200, nil
}
func (a generatedAPI) Skip(ctx context.Context, h, id string) (*daikuv1.RecurringOccurrence, error) {
	r, e := a.c.DaikuHouseholdsHouseholdPkRecurringOccurrencesIdSkipPostWithResponse(ctx, h, id)
	if e != nil {
		return nil, apiFailure()
	}
	if e = status(r.StatusCode()); e != nil {
		return nil, e
	}
	return r.JSON200, nil
}
func (a generatedAPI) Snooze(ctx context.Context, h, id string, b daikuv1.RecurringOccurrenceSnoozeRequestRequest) (*daikuv1.RecurringOccurrence, error) {
	r, e := a.c.DaikuHouseholdsHouseholdPkRecurringOccurrencesIdSnoozePostWithResponse(ctx, h, id, b)
	if e != nil {
		return nil, apiFailure()
	}
	if e = status(r.StatusCode()); e != nil {
		return nil, e
	}
	return r.JSON200, nil
}
