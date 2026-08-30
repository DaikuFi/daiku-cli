package transactions

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"time"

	"github.com/DaikuFi/daiku-cli/generated/daikuv1"
	"github.com/DaikuFi/daiku-cli/internal/cli"
	"github.com/DaikuFi/daiku-cli/internal/output"
	"github.com/DaikuFi/daiku-cli/internal/prompt"
	openapi_types "github.com/oapi-codegen/runtime/types"
	"github.com/spf13/cobra"
)

var positiveMoneyPattern = regexp.MustCompile(`^(?:0\.(?:0[1-9]|[1-9][0-9]?)|[1-9][0-9]*(?:\.[0-9]{1,2})?)$`)
var signedMoneyPattern = regexp.MustCompile(`^-?[0-9]{1,10}(?:\.[0-9]{1,2})?$`)

type Module struct{ Factory ServiceFactory }

func New(factory ServiceFactory) Module { return Module{Factory: factory} }
func (m Module) Register(root *cobra.Command) {
	tx := &cobra.Command{Use: "transactions", Short: "Manage transactions", Args: cli.UsageArgs(cobra.NoArgs)}
	tx.AddCommand(m.list(false), m.list(true), m.create(), m.update(), m.delete(), m.bulkCreate(), m.bulkUpdate(), m.bulkDelete())
	tr := &cobra.Command{Use: "transfers", Short: "Manage transfers", Args: cli.UsageArgs(cobra.NoArgs)}
	tr.AddCommand(m.transferCreate(), m.transferConvert(), m.transferCandidates(), m.transferUnlink())
	ins := &cobra.Command{Use: "installments", Short: "Manage installment plans", Args: cli.UsageArgs(cobra.NoArgs)}
	ins.AddCommand(m.installmentCreate(), m.installmentGet(), m.installmentUpdate())
	root.AddCommand(tx, tr, ins)
}

func requiredString(cmd *cobra.Command, name, usage string) *string {
	value := new(string)
	cmd.Flags().StringVar(value, name, "", usage)
	_ = cmd.MarkFlagRequired(name)
	return value
}
func optionalString(cmd *cobra.Command, name, usage string) *string {
	value := new(string)
	cmd.Flags().StringVar(value, name, "", usage)
	return value
}
func service(m Module) (Service, error) {
	if m.Factory == nil {
		return nil, safe("client_error", "transaction service is not configured")
	}
	return m.Factory()
}
func dateValue(raw string) (*openapi_types.Date, error) {
	if raw == "" {
		return nil, nil
	}
	parsed, err := time.Parse("2006-01-02", raw)
	if err != nil {
		return nil, &cli.Error{Code: "invalid_date", Message: "date must use YYYY-MM-DD", ExitCode: cli.ExitUsage}
	}
	v := openapi_types.Date{Time: parsed}
	return &v, nil
}
func decimal(raw string, required bool) (*string, error) {
	if raw == "" && !required {
		return nil, nil
	}
	if !positiveMoneyPattern.MatchString(raw) {
		return nil, &cli.Error{Code: "invalid_amount", Message: "amount must be a positive decimal string with at most two fractional digits", ExitCode: cli.ExitUsage}
	}
	return &raw, nil
}
func expenseDecimal(raw string, required bool) (*string, error) {
	if raw == "" && !required {
		return nil, nil
	}
	if !signedMoneyPattern.MatchString(raw) {
		return nil, &cli.Error{Code: "invalid_amount", Message: "amount must be a decimal string with at most ten whole and two fractional digits", ExitCode: cli.ExitUsage}
	}
	return &raw, nil
}
func currency(raw string) (*daikuv1.Currency3e8Enum, error) {
	if raw == "" {
		return nil, nil
	}
	if raw != "UYU" && raw != "USD" && raw != "EUR" {
		return nil, &cli.Error{Code: "invalid_currency", Message: "currency must be UYU, USD, or EUR", ExitCode: cli.ExitUsage}
	}
	v := daikuv1.Currency3e8Enum(raw)
	return &v, nil
}
func printResult(cmd *cobra.Command, value any) error {
	jsonOut, _ := cmd.Flags().GetBool("json")
	if jsonOut {
		return cli.WriteSuccess(cmd.OutOrStdout(), value)
	}
	items := expenses(value)
	human := cli.Human(cmd)
	renderer := output.Renderer{Writer: cmd.OutOrStdout(), Localize: human.Localizer, Terminal: human.Terminal, Width: human.Width, NoColor: human.NoColor}
	if items != nil {
		return renderer.Table(expenseRows(items, human.Localizer.Language == "es"))
	}
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(cmd.OutOrStdout(), string(data))
	return err
}
func expenses(value any) []daikuv1.Expense {
	switch v := value.(type) {
	case daikuv1.Expense:
		return []daikuv1.Expense{v}
	case []daikuv1.Expense:
		return v
	case daikuv1.ExpensePage:
		return v.Results
	case daikuv1.TransferResponse:
		return v.Transactions
	case daikuv1.TransferUnlinkResponse:
		return v.Transactions
	}
	return nil
}
func expenseRows(items []daikuv1.Expense, es bool) []output.Row {
	labels := []string{"ID", "Date", "Description", "Amount", "Currency", "Type"}
	if es {
		labels = []string{"ID", "Fecha", "Descripción", "Importe", "Moneda", "Tipo"}
	}
	rows := make([]output.Row, 0, len(items))
	for _, item := range items {
		date := ""
		if item.ExpenseDate != nil {
			date = item.ExpenseDate.Format("2006-01-02")
		}
		id := value(item.Id)
		cur := ""
		if item.Currency != nil {
			cur = string(*item.Currency)
		}
		kind := ""
		if item.TransactionType != nil {
			kind = string(*item.TransactionType)
		}
		rows = append(rows, output.Row{{Label: labels[0], Value: id}, {Label: labels[1], Value: date}, {Label: labels[2], Value: item.Description}, {Label: labels[3], Value: item.Amount}, {Label: labels[4], Value: cur}, {Label: labels[5], Value: kind}})
	}
	return rows
}
func value(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}
func confirm(cmd *cobra.Command, yes bool, action string) error {
	if yes {
		return nil
	}
	h := cli.Human(cmd)
	p := prompt.Prompter{In: cmd.InOrStdin(), Out: cmd.ErrOrStderr(), Localize: h.Localizer, Terminal: h.Interactive && !h.JSON}
	err := p.ConfirmDestructive(h.Localizer.Human(action))
	if errors.Is(err, prompt.ErrNonInteractive) {
		return &cli.Error{Code: "confirmation_required", Message: "confirmation requires an interactive terminal; pass --yes to continue", ExitCode: cli.ExitUsage}
	}
	if errors.Is(err, prompt.ErrAborted) {
		return &cli.Error{Code: "operation_cancelled", Message: "operation cancelled", ExitCode: cli.ExitConflict}
	}
	return err
}

func (m Module) list(search bool) *cobra.Command {
	use := "list"
	short := "List transactions"
	if search {
		use = "search"
		short = "Search transactions"
	}
	cmd := &cobra.Command{Use: use, Short: short, Args: cli.UsageArgs(cobra.NoArgs)}
	hh := requiredString(cmd, "household", "household ID")
	q := optionalString(cmd, "query", "search query")
	from := optionalString(cmd, "from", "inclusive start date")
	to := optionalString(cmd, "to", "inclusive end date")
	account := optionalString(cmd, "account", "account ID")
	category := optionalString(cmd, "category", "category ID")
	kind := optionalString(cmd, "kind", "recurring or one-time")
	typ := optionalString(cmd, "type", "expense or income")
	ordering := optionalString(cmd, "ordering", "newest, oldest, amount_high, or amount_low")
	var tags []string
	cmd.Flags().StringSliceVar(&tags, "tag", nil, "tag ID (repeatable)")
	cmd.RunE = func(cmd *cobra.Command, _ []string) error {
		if search && *q == "" {
			return &cli.Error{Code: "query_required", Message: "--query is required", ExitCode: cli.ExitUsage}
		}
		fd, e := dateValue(*from)
		if e != nil {
			return e
		}
		td, e := dateValue(*to)
		if e != nil {
			return e
		}
		if fd != nil && td != nil && fd.Time.After(td.Time) {
			return &cli.Error{Code: "invalid_range", Message: "--from must not be after --to", ExitCode: cli.ExitUsage}
		}
		p := &daikuv1.DaikuHouseholdsHouseholdPkExpensesGetParams{DateFrom: fd, DateTo: td, Paginated: boolptr(true)}
		if *q != "" {
			p.Q = q
		}
		if *account != "" {
			p.Account = account
		}
		if *category != "" {
			p.Category = category
		}
		if *kind != "" {
			if *kind != "recurring" && *kind != "one-time" {
				return usage("kind must be recurring or one-time")
			}
			p.Kind = kind
		}
		if *typ != "" {
			if *typ != "expense" && *typ != "income" {
				return usage("type must be expense or income")
			}
			p.Type = typ
		}
		if *ordering != "" {
			allowed := map[string]bool{"newest": true, "oldest": true, "amount_high": true, "amount_low": true}
			if !allowed[*ordering] {
				return usage("invalid ordering")
			}
			p.Ordering = ordering
		}
		if len(tags) > 0 {
			p.Tag = &tags
		}
		s, e := service(m)
		if e != nil {
			return e
		}
		v, e := s.List(cmd.Context(), *hh, p)
		if e != nil {
			return e
		}
		return printResult(cmd, v)
	}
	return cmd
}

func expenseFlags(cmd *cobra.Command, required bool) (amount, accountAmount, description, cur, date, account, category *string, tags *[]string) {
	if required {
		amount = requiredString(cmd, "amount", "decimal amount")
		description = requiredString(cmd, "description", "description")
	} else {
		amount = optionalString(cmd, "amount", "decimal amount")
		description = optionalString(cmd, "description", "description")
	}
	accountAmount = optionalString(cmd, "account-amount", "amount posted to the selected account")
	cur = optionalString(cmd, "currency", "UYU, USD, or EUR")
	date = optionalString(cmd, "date", "transaction date (YYYY-MM-DD)")
	account = optionalString(cmd, "account", "account ID")
	category = optionalString(cmd, "category", "category ID")
	tags = new([]string)
	cmd.Flags().StringSliceVar(tags, "tag", nil, "tag ID (repeatable)")
	return
}
func (m Module) create() *cobra.Command {
	cmd := &cobra.Command{Use: "create", Short: "Create a transaction", Args: cli.UsageArgs(cobra.NoArgs)}
	hh := requiredString(cmd, "household", "household ID")
	amount, accountAmount, description, cur, date, account, category, tags := expenseFlags(cmd, true)
	kind := optionalString(cmd, "type", "expense or income")
	cmd.RunE = func(cmd *cobra.Command, _ []string) error {
		a, e := expenseDecimal(*amount, true)
		if e != nil {
			return e
		}
		c, e := currency(*cur)
		if e != nil {
			return e
		}
		d, e := dateValue(*date)
		if e != nil {
			return e
		}
		b := daikuv1.ExpenseRequest{Amount: *a, Description: *description, Account: nil, Category: nil, RecurringExpense: nil, Currency: c, ExpenseDate: d}
		if *accountAmount != "" {
			aa, err := expenseDecimal(*accountAmount, true)
			if err != nil {
				return err
			}
			b.AccountAmount = aa
		}
		if *account != "" {
			b.Account = account
		}
		if *category != "" {
			b.Category = category
		}
		if len(*tags) > 0 {
			b.TagIds = tags
		}
		if *kind != "" {
			if *kind != "expense" && *kind != "income" {
				return usage("type must be expense or income")
			}
			v := daikuv1.ExpenseTransactionTypeEnum(*kind)
			b.TransactionType = &v
		}
		s, e := service(m)
		if e != nil {
			return e
		}
		v, e := s.Create(cmd.Context(), *hh, b)
		if e != nil {
			return e
		}
		return printResult(cmd, v)
	}
	return cmd
}
func (m Module) update() *cobra.Command {
	cmd := &cobra.Command{Use: "update ID", Short: "Update a transaction", Args: cli.UsageArgs(cobra.ExactArgs(1))}
	hh := requiredString(cmd, "household", "household ID")
	amount, accountAmount, description, cur, date, account, category, tags := expenseFlags(cmd, false)
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		b := daikuv1.PatchedExpenseRequest{Account: nil, Category: nil, RecurringExpense: nil}
		changed := false
		if *accountAmount != "" {
			aa, err := expenseDecimal(*accountAmount, true)
			if err != nil {
				return err
			}
			b.AccountAmount = aa
			changed = true
		}
		if *amount != "" {
			a, e := expenseDecimal(*amount, true)
			if e != nil {
				return e
			}
			b.Amount = a
			changed = true
		}
		if *description != "" {
			b.Description = description
			changed = true
		}
		if *cur != "" {
			c, e := currency(*cur)
			if e != nil {
				return e
			}
			b.Currency = c
			changed = true
		}
		if *date != "" {
			d, e := dateValue(*date)
			if e != nil {
				return e
			}
			b.ExpenseDate = d
			changed = true
		}
		if *account != "" {
			b.Account = account
			changed = true
		}
		if *category != "" {
			b.Category = category
			changed = true
		}
		if len(*tags) > 0 {
			b.TagIds = tags
			changed = true
		}
		if !changed {
			return usage("at least one update flag is required")
		}
		s, e := service(m)
		if e != nil {
			return e
		}
		v, e := s.Update(cmd.Context(), *hh, args[0], b)
		if e != nil {
			return e
		}
		return printResult(cmd, v)
	}
	return cmd
}
func (m Module) delete() *cobra.Command {
	var yes bool
	cmd := &cobra.Command{Use: "delete ID", Short: "Delete a transaction", Args: cli.UsageArgs(cobra.ExactArgs(1))}
	hh := requiredString(cmd, "household", "household ID")
	scope := optionalString(cmd, "scope", "this, future, or plan for installments")
	cmd.Flags().BoolVar(&yes, "yes", false, "skip the interactive confirmation")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		if *scope != "" && *scope != "this" && *scope != "future" && *scope != "plan" {
			return usage("scope must be this, future, or plan")
		}
		if e := confirm(cmd, yes, "Delete transaction "+args[0]+"."); e != nil {
			return e
		}
		p := &daikuv1.DaikuHouseholdsHouseholdPkExpensesIdDeleteParams{}
		if *scope != "" {
			p.Scope = scope
		}
		s, e := service(m)
		if e != nil {
			return e
		}
		if e = s.Delete(cmd.Context(), *hh, args[0], p); e != nil {
			return e
		}
		return printResult(cmd, map[string]any{"deleted": true, "id": args[0], "scope": *scope})
	}
	return cmd
}

func readJSON(cmd *cobra.Command, path string, out any) error {
	var r io.Reader
	if path == "-" {
		r = cmd.InOrStdin()
	} else {
		f, e := os.Open(path)
		if e != nil {
			return &cli.Error{Code: "input_error", Message: "input file could not be opened", ExitCode: cli.ExitUsage}
		}
		defer f.Close()
		r = f
	}
	dec := json.NewDecoder(r)
	dec.DisallowUnknownFields()
	if e := dec.Decode(out); e != nil {
		return &cli.Error{Code: "invalid_json", Message: "input must be valid JSON matching the API contract", ExitCode: cli.ExitUsage}
	}
	if e := dec.Decode(&struct{}{}); e != io.EOF {
		return &cli.Error{Code: "invalid_json", Message: "input must contain exactly one JSON value", ExitCode: cli.ExitUsage}
	}
	return nil
}
func (m Module) bulkCreate() *cobra.Command {
	cmd := &cobra.Command{Use: "bulk-create", Short: "Create transactions in bulk", Args: cli.UsageArgs(cobra.NoArgs)}
	hh := requiredString(cmd, "household", "household ID")
	file := requiredString(cmd, "file", "JSON file, or - for stdin")
	cmd.RunE = func(cmd *cobra.Command, _ []string) error {
		var b daikuv1.ExpenseBulkCreateRequestRequest
		if e := readJSON(cmd, *file, &b); e != nil {
			return e
		}
		if len(b.Expenses) == 0 {
			return usage("expenses must not be empty")
		}
		for _, x := range b.Expenses {
			if _, e := expenseDecimal(x.Amount, true); e != nil {
				return e
			}
			if x.Currency != nil {
				if _, e := currency(string(*x.Currency)); e != nil {
					return e
				}
			}
		}
		s, e := service(m)
		if e != nil {
			return e
		}
		v, e := s.BulkCreate(cmd.Context(), *hh, b)
		if e != nil {
			return e
		}
		return printResult(cmd, v)
	}
	return cmd
}
func (m Module) bulkUpdate() *cobra.Command {
	var yes bool
	cmd := &cobra.Command{Use: "bulk-update", Short: "Update matching transactions in bulk", Args: cli.UsageArgs(cobra.NoArgs)}
	hh := requiredString(cmd, "household", "household ID")
	file := requiredString(cmd, "file", "JSON file, or - for stdin")
	cmd.Flags().BoolVar(&yes, "yes", false, "skip the interactive confirmation")
	cmd.RunE = func(cmd *cobra.Command, _ []string) error {
		var b daikuv1.PatchedExpenseBulkUpdateRequestRequest
		if e := readJSON(cmd, *file, &b); e != nil {
			return e
		}
		if b.Ids == nil || len(*b.Ids) == 0 || b.Updates == nil {
			return usage("ids and updates are required")
		}
		if e := confirm(cmd, yes, fmt.Sprintf("Update %d transactions.", len(*b.Ids))); e != nil {
			return e
		}
		s, e := service(m)
		if e != nil {
			return e
		}
		v, e := s.BulkUpdate(cmd.Context(), *hh, b)
		if e != nil {
			return e
		}
		return printResult(cmd, v)
	}
	return cmd
}

func (m Module) bulkDelete() *cobra.Command {
	var yes bool
	cmd := &cobra.Command{Use: "delete-all", Short: "Delete all transactions", Args: cli.UsageArgs(cobra.NoArgs)}
	hh := requiredString(cmd, "household", "household ID")
	cmd.Flags().BoolVar(&yes, "yes", false, "skip the interactive confirmation")
	cmd.RunE = func(cmd *cobra.Command, _ []string) error {
		if e := confirm(cmd, yes, "Delete every transaction in household "+*hh+"."); e != nil {
			return e
		}
		s, e := service(m)
		if e != nil {
			return e
		}
		v, e := s.BulkDelete(cmd.Context(), *hh)
		if e != nil {
			return e
		}
		return printResult(cmd, v)
	}
	return cmd
}

func (m Module) transferCreate() *cobra.Command {
	cmd := &cobra.Command{Use: "create", Short: "Create a balanced transfer", Args: cli.UsageArgs(cobra.NoArgs)}
	hh := requiredString(cmd, "household", "household ID")
	from := requiredString(cmd, "from-account", "source account ID")
	to := requiredString(cmd, "to-account", "destination account ID")
	amount := requiredString(cmd, "amount", "source decimal amount")
	toAmount := optionalString(cmd, "to-amount", "destination decimal amount")
	date := optionalString(cmd, "date", "transfer date")
	description := optionalString(cmd, "description", "description")
	cmd.RunE = func(cmd *cobra.Command, _ []string) error {
		if *from == *to {
			return usage("source and destination accounts must differ")
		}
		a, e := decimal(*amount, true)
		if e != nil {
			return e
		}
		ta, e := decimal(*toAmount, false)
		if e != nil {
			return e
		}
		d, e := dateValue(*date)
		if e != nil {
			return e
		}
		b := daikuv1.TransferCreateRequestRequest{Amount: *a, FromAccount: *from, ToAccount: *to, ToAmount: ta, Date: d}
		if *description != "" {
			b.Description = description
		}
		s, e := service(m)
		if e != nil {
			return e
		}
		v, e := s.CreateTransfer(cmd.Context(), *hh, b)
		if e != nil {
			return e
		}
		return printResult(cmd, v)
	}
	return cmd
}
func (m Module) transferConvert() *cobra.Command {
	var yes bool
	cmd := &cobra.Command{Use: "convert TRANSACTION_ID", Short: "Convert a transaction to a transfer", Args: cli.UsageArgs(cobra.ExactArgs(1))}
	hh := requiredString(cmd, "household", "household ID")
	to := optionalString(cmd, "to-account", "destination account ID")
	peer := optionalString(cmd, "peer", "existing peer transaction ID")
	peerAmount := optionalString(cmd, "peer-amount", "peer decimal amount")
	cmd.Flags().BoolVar(&yes, "yes", false, "skip the interactive confirmation")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		if (*to == "") == (*peer == "") {
			return usage("exactly one of --to-account or --peer is required")
		}
		if e := confirm(cmd, yes, "Convert transaction "+args[0]+" to a transfer."); e != nil {
			return e
		}
		pa, e := decimal(*peerAmount, false)
		if e != nil {
			return e
		}
		b := daikuv1.TransferConvertRequestRequest{PeerAmount: pa}
		if *to != "" {
			b.ToAccount = to
		}
		if *peer != "" {
			b.Peer = peer
		}
		s, e := service(m)
		if e != nil {
			return e
		}
		v, e := s.ConvertTransfer(cmd.Context(), *hh, args[0], b)
		if e != nil {
			return e
		}
		return printResult(cmd, v)
	}
	return cmd
}
func (m Module) transferCandidates() *cobra.Command {
	cmd := &cobra.Command{Use: "candidates TRANSACTION_ID", Short: "List transfer candidates", Args: cli.UsageArgs(cobra.ExactArgs(1))}
	hh := requiredString(cmd, "household", "household ID")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		s, e := service(m)
		if e != nil {
			return e
		}
		v, e := s.TransferCandidates(cmd.Context(), *hh, args[0])
		if e != nil {
			return e
		}
		return printResult(cmd, v)
	}
	return cmd
}
func (m Module) transferUnlink() *cobra.Command {
	var yes bool
	cmd := &cobra.Command{Use: "unlink TRANSACTION_ID", Short: "Unlink both transfer legs", Args: cli.UsageArgs(cobra.ExactArgs(1))}
	hh := requiredString(cmd, "household", "household ID")
	cmd.Flags().BoolVar(&yes, "yes", false, "skip the interactive confirmation")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		if e := confirm(cmd, yes, "Unlink transfer "+args[0]+"."); e != nil {
			return e
		}
		s, e := service(m)
		if e != nil {
			return e
		}
		v, e := s.UnlinkTransfer(cmd.Context(), *hh, args[0])
		if e != nil {
			return e
		}
		return printResult(cmd, v)
	}
	return cmd
}

func (m Module) installmentCreate() *cobra.Command {
	cmd := &cobra.Command{Use: "create", Short: "Create an installment plan", Args: cli.UsageArgs(cobra.NoArgs)}
	hh := requiredString(cmd, "household", "household ID")
	amount := requiredString(cmd, "amount", "purchase total as decimal string")
	description := requiredString(cmd, "description", "description")
	cur := requiredString(cmd, "currency", "UYU, USD, or EUR")
	date := requiredString(cmd, "date", "purchase date")
	var count int
	cmd.Flags().IntVar(&count, "count", 0, "number of installments")
	_ = cmd.MarkFlagRequired("count")
	account := optionalString(cmd, "account", "account ID")
	accountAmount := optionalString(cmd, "account-amount", "amount posted to the selected account")
	category := optionalString(cmd, "category", "category ID")
	cmd.RunE = func(cmd *cobra.Command, _ []string) error {
		a, e := decimal(*amount, true)
		if e != nil {
			return e
		}
		c, e := currency(*cur)
		if e != nil {
			return e
		}
		d, e := dateValue(*date)
		if e != nil {
			return e
		}
		if count < 2 || count > 60 {
			return usage("count must be between 2 and 60")
		}
		b := daikuv1.InstallmentCreateRequestRequest{Amount: *a, Description: *description, Currency: daikuv1.Currency43eEnum(*c), ExpenseDate: *d, Installments: count, Account: nil, Category: nil}
		if *accountAmount != "" {
			aa, err := expenseDecimal(*accountAmount, true)
			if err != nil {
				return err
			}
			b.AccountAmount = aa
		}
		if *account != "" {
			b.Account = account
		}
		if *category != "" {
			b.Category = category
		}
		s, e := service(m)
		if e != nil {
			return e
		}
		v, e := s.CreateInstallments(cmd.Context(), *hh, b)
		if e != nil {
			return e
		}
		return printResult(cmd, v)
	}
	return cmd
}
func (m Module) installmentGet() *cobra.Command {
	cmd := &cobra.Command{Use: "get ID", Short: "Show an installment plan", Args: cli.UsageArgs(cobra.ExactArgs(1))}
	hh := requiredString(cmd, "household", "household ID")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		s, e := service(m)
		if e != nil {
			return e
		}
		v, e := s.GetInstallment(cmd.Context(), *hh, args[0])
		if e != nil {
			return e
		}
		return printResult(cmd, v)
	}
	return cmd
}
func (m Module) installmentUpdate() *cobra.Command {
	cmd := &cobra.Command{Use: "update ID", Short: "Update an installment plan", Args: cli.UsageArgs(cobra.ExactArgs(1))}
	hh := requiredString(cmd, "household", "household ID")
	amount := optionalString(cmd, "amount", "purchase total, never cuota amount")
	description := optionalString(cmd, "description", "description")
	cur := optionalString(cmd, "currency", "UYU, USD, or EUR")
	account := optionalString(cmd, "account", "account ID")
	category := optionalString(cmd, "category", "category ID")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		b := daikuv1.PatchedInstallmentPlanUpdateRequest{Account: nil, Category: nil}
		changed := false
		if *amount != "" {
			a, e := decimal(*amount, true)
			if e != nil {
				return e
			}
			b.Amount = a
			changed = true
		}
		if *description != "" {
			b.Description = description
			changed = true
		}
		if *cur != "" {
			if _, e := currency(*cur); e != nil {
				return e
			}
			b.Currency = cur
			changed = true
		}
		if *account != "" {
			b.Account = account
			changed = true
		}
		if *category != "" {
			b.Category = category
			changed = true
		}
		if !changed {
			return usage("at least one update flag is required")
		}
		s, e := service(m)
		if e != nil {
			return e
		}
		v, e := s.UpdateInstallment(cmd.Context(), *hh, args[0], b)
		if e != nil {
			return e
		}
		return printResult(cmd, v)
	}
	return cmd
}

func boolptr(v bool) *bool { return &v }
func usage(message string) *cli.Error {
	return &cli.Error{Code: "usage_error", Message: message, ExitCode: cli.ExitUsage}
}
