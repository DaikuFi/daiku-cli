package portfolios

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	daikuv1 "github.com/DaikuFi/daiku-cli/generated/daikuv1"
	"github.com/DaikuFi/daiku-cli/internal/cli"
	"github.com/DaikuFi/daiku-cli/internal/i18n"
	"github.com/DaikuFi/daiku-cli/internal/output"
	"github.com/DaikuFi/daiku-cli/internal/prompt"
	openapi_types "github.com/oapi-codegen/runtime/types"
	"github.com/spf13/cobra"
)

type Module struct{ Factory Factory }

func New(factory Factory) Module { return Module{Factory: factory} }
func (m Module) Register(root *cobra.Command) {
	p := &cobra.Command{Use: "portfolios", Short: "Manage portfolios and inspect server-calculated totals", Args: cli.UsageArgs(cobra.NoArgs)}
	p.AddCommand(m.portfolioList(), m.portfolioGet(), m.portfolioCreate(), m.portfolioUpdate(), m.portfolioDelete(), m.totals(), m.holdings(), m.buckets())
	root.AddCommand(p)
	root.AddCommand(m.assets())
}
func (m Module) service(cmd *cobra.Command) (Service, error) { return m.Factory(cmd.Context()) }
func run(use, short string, args cobra.PositionalArgs, fn func(*cobra.Command, []string) error) *cobra.Command {
	return &cobra.Command{Use: use, Short: short, Args: cli.UsageArgs(args), RunE: fn}
}
func commandError(code, message string, exit cli.ExitCode) *cli.Error {
	return &cli.Error{Code: code, Message: message, ExitCode: exit}
}
func usage(message string) error { return commandError("usage_error", message, cli.ExitUsage) }
func emit(cmd *cobra.Command, data any, human string, args ...any) error {
	jsonOut, _ := cmd.Flags().GetBool("json")
	if jsonOut {
		return cli.WriteSuccess(cmd.OutOrStdout(), data)
	}
	if list, ok := data.([]daikuv1.PortfolioList); ok && len(list) == 0 {
		_, e := fmt.Fprintln(cmd.OutOrStdout(), cli.Human(cmd).Localizer.Text(i18n.NoResults))
		return e
	}
	_, e := fmt.Fprintf(cmd.OutOrStdout(), humanText(cmd, human), args...)
	return e
}
func emitObject(cmd *cobra.Command, data any, label string) error {
	jsonOut, _ := cmd.Flags().GetBool("json")
	if jsonOut {
		return cli.WriteSuccess(cmd.OutOrStdout(), data)
	}
	var decoded any
	encoded, err := json.Marshal(data)
	if err != nil {
		return err
	}
	if err = json.Unmarshal(encoded, &decoded); err != nil {
		return err
	}
	rows := humanRows(decoded)
	if _, err = fmt.Fprintln(cmd.OutOrStdout(), humanText(cmd, label)); err != nil {
		return err
	}
	if len(rows) == 0 {
		_, err = fmt.Fprintln(cmd.OutOrStdout(), cli.Human(cmd).Localizer.Text(i18n.NoResults))
		return err
	}
	h := cli.Human(cmd)
	return (output.Renderer{Writer: cmd.OutOrStdout(), Localize: h.Localizer, Terminal: h.Terminal, Width: h.Width, NoColor: h.NoColor}).Table(rows)
}

func humanRows(value any) []output.Row {
	if wrapper, ok := value.(map[string]any); ok && len(wrapper) == 1 {
		for _, nested := range wrapper {
			if list, ok := nested.([]any); ok {
				value = list
			}
		}
	}
	objects := []map[string]any{}
	switch typed := value.(type) {
	case []any:
		for _, item := range typed {
			if object, ok := item.(map[string]any); ok {
				objects = append(objects, object)
			}
		}
	case map[string]any:
		objects = append(objects, typed)
	}
	rows := make([]output.Row, 0, len(objects))
	for _, object := range objects {
		keys := make([]string, 0, len(object))
		for key, item := range object {
			if item != nil {
				keys = append(keys, key)
			}
		}
		sort.Strings(keys)
		row := make(output.Row, 0, len(keys))
		for _, key := range keys {
			item := object[key]
			rendered := fmt.Sprint(item)
			switch item.(type) {
			case []any, map[string]any:
				nested, _ := json.Marshal(item)
				rendered = string(nested)
			}
			row = append(row, output.Cell{Label: strings.ToUpper(strings.ReplaceAll(key, "_", " ")), Value: rendered})
		}
		rows = append(rows, row)
	}
	return rows
}

func humanText(cmd *cobra.Command, english string) string {
	if cli.Human(cmd).Localizer.Language != i18n.Spanish {
		return english
	}
	translations := map[string]string{
		"Portfolio": "Portafolio", "Portfolio created": "Portafolio creado", "Portfolio updated": "Portafolio actualizado",
		"Portfolio totals (calculated by Daiku)": "Totales del portafolio (calculados por Daiku)", "Portfolio holdings (calculated by Daiku)": "Tenencias del portafolio (calculadas por Daiku)",
		"Buckets": "Grupos", "Bucket created": "Grupo creado", "Bucket updated": "Grupo actualizado", "Assets": "Activos", "Asset created": "Activo creado", "Asset updated": "Activo actualizado",
		"Cashflows": "Flujos de caja", "Cashflow created": "Flujo de caja creado", "Cashflow updated": "Flujo de caja actualizado", "Value history": "Historial de valor",
		"Value-history point created": "Punto del historial de valor creado", "Value-history point updated": "Punto del historial de valor actualizado", "No results.": "No hay resultados.",
		"%d portfolios.\n": "%d portafolios.\n", "Portfolio %s deleted.\n": "Portafolio %s eliminado.\n", "Bucket %s deleted.\n": "Grupo %s eliminado.\n",
		"Asset %s deleted.\n": "Activo %s eliminado.\n", "Cashflow %s deleted.\n": "Flujo de caja %s eliminado.\n", "Value-history point %s deleted.\n": "Punto del historial de valor %s eliminado.\n",
	}
	for prefix, translated := range map[string]string{"Delete portfolio ": "Eliminar portafolio ", "Delete bucket ": "Eliminar grupo ", "Delete asset ": "Eliminar activo ", "Delete cashflow ": "Eliminar flujo de caja ", "Delete value-history point ": "Eliminar punto del historial de valor "} {
		if strings.HasPrefix(english, prefix) {
			return translated + strings.TrimPrefix(english, prefix)
		}
	}
	if translated, ok := translations[english]; ok {
		return translated
	}
	return english
}
func confirm(cmd *cobra.Command, yes bool, action string) error {
	if yes {
		return nil
	}
	h := cli.Human(cmd)
	p := prompt.Prompter{In: cmd.InOrStdin(), Out: cmd.ErrOrStderr(), Localize: h.Localizer, Terminal: h.Interactive && !h.JSON}
	if err := p.ConfirmDestructive(humanText(cmd, action)); err != nil {
		if errors.Is(err, prompt.ErrNonInteractive) {
			return commandError("confirmation_required", "confirmation requires an interactive terminal; pass --yes to continue", cli.ExitUsage)
		}
		if errors.Is(err, prompt.ErrAborted) {
			return commandError("operation_cancelled", "operation cancelled", cli.ExitConflict)
		}
		return commandError("confirmation_failed", "confirmation could not be read", cli.ExitFailure)
	}
	return nil
}
func validCurrency(v string) bool { return v == "UYU" || v == "USD" || v == "EUR" }

func (m Module) portfolioList() *cobra.Command {
	return run("list", "List portfolios", cobra.NoArgs, func(c *cobra.Command, _ []string) error {
		s, e := m.service(c)
		if e != nil {
			return e
		}
		v, e := s.PortfolioList(c.Context())
		if e != nil {
			return e
		}
		return emit(c, v, "%d portfolios.\n", len(v))
	})
}
func (m Module) portfolioGet() *cobra.Command {
	return run("get <id>", "Get a portfolio", cobra.ExactArgs(1), func(c *cobra.Command, a []string) error {
		s, e := m.service(c)
		if e != nil {
			return e
		}
		v, e := s.PortfolioGet(c.Context(), a[0])
		if e != nil {
			return e
		}
		return emitObject(c, v, "Portfolio")
	})
}

type portfolioFlags struct {
	name, currency, emoji string
	isDefault             bool
}

func addPortfolioFlags(c *cobra.Command, f *portfolioFlags, create bool) {
	c.Flags().StringVar(&f.name, "name", "", "portfolio name")
	c.Flags().StringVar(&f.currency, "display-currency", "", "display currency: UYU, USD or EUR")
	c.Flags().StringVar(&f.emoji, "emoji", "", "portfolio emoji")
	c.Flags().BoolVar(&f.isDefault, "default", false, "make this the default portfolio")
	if create {
		_ = c.MarkFlagRequired("name")
	}
}
func (m Module) portfolioCreate() *cobra.Command {
	var f portfolioFlags
	c := run("create", "Create a portfolio", cobra.NoArgs, func(c *cobra.Command, _ []string) error {
		if f.currency != "" && !validCurrency(f.currency) {
			return usage("display currency must be UYU, USD or EUR")
		}
		b := daikuv1.PortfolioListRequest{Name: f.name}
		if f.currency != "" {
			v := daikuv1.DisplayCurrency3e8Enum(f.currency)
			b.DisplayCurrency = &v
		}
		if f.emoji != "" {
			b.Emoji = &f.emoji
		}
		if c.Flags().Changed("default") {
			b.IsDefault = &f.isDefault
		}
		s, e := m.service(c)
		if e != nil {
			return e
		}
		v, e := s.PortfolioCreate(c.Context(), b)
		if e != nil {
			return e
		}
		return emitObject(c, v, "Portfolio created")
	})
	addPortfolioFlags(c, &f, true)
	return c
}
func (m Module) portfolioUpdate() *cobra.Command {
	var f portfolioFlags
	c := run("update <id>", "Update a portfolio", cobra.ExactArgs(1), func(c *cobra.Command, a []string) error {
		if !anyChanged(c, "name", "display-currency", "emoji", "default") {
			return usage("provide at least one field to update")
		}
		if f.currency != "" && !validCurrency(f.currency) {
			return usage("display currency must be UYU, USD or EUR")
		}
		b := daikuv1.PatchedPortfolioListRequest{}
		if c.Flags().Changed("name") {
			b.Name = &f.name
		}
		if c.Flags().Changed("display-currency") {
			v := daikuv1.DisplayCurrency3e8Enum(f.currency)
			b.DisplayCurrency = &v
		}
		if c.Flags().Changed("emoji") {
			b.Emoji = &f.emoji
		}
		if c.Flags().Changed("default") {
			b.IsDefault = &f.isDefault
		}
		s, e := m.service(c)
		if e != nil {
			return e
		}
		v, e := s.PortfolioUpdate(c.Context(), a[0], b)
		if e != nil {
			return e
		}
		return emitObject(c, v, "Portfolio updated")
	})
	addPortfolioFlags(c, &f, false)
	return c
}
func (m Module) portfolioDelete() *cobra.Command {
	var yes bool
	c := run("delete <id>", "Delete a portfolio", cobra.ExactArgs(1), func(c *cobra.Command, a []string) error {
		if e := confirm(c, yes, "Delete portfolio "+a[0]+"."); e != nil {
			return e
		}
		s, e := m.service(c)
		if e != nil {
			return e
		}
		if e = s.PortfolioDelete(c.Context(), a[0]); e != nil {
			return e
		}
		return emit(c, map[string]any{"deleted": true, "id": a[0]}, "Portfolio %s deleted.\n", a[0])
	})
	c.Flags().BoolVar(&yes, "yes", false, "skip the interactive confirmation")
	return c
}
func (m Module) totals() *cobra.Command {
	return run("totals <id>", "Show server-calculated assets, liabilities and net worth", cobra.ExactArgs(1), func(c *cobra.Command, a []string) error {
		s, e := m.service(c)
		if e != nil {
			return e
		}
		v, e := s.Totals(c.Context(), a[0])
		if e != nil {
			return e
		}
		return emitObject(c, v, "Portfolio totals (calculated by Daiku)")
	})
}
func (m Module) holdings() *cobra.Command {
	return run("holdings <id>", "Show server-calculated portfolio holdings", cobra.ExactArgs(1), func(c *cobra.Command, a []string) error {
		s, e := m.service(c)
		if e != nil {
			return e
		}
		v, e := s.Holdings(c.Context(), a[0])
		if e != nil {
			return e
		}
		return emitObject(c, v, "Portfolio holdings (calculated by Daiku)")
	})
}

func (m Module) buckets() *cobra.Command {
	c := &cobra.Command{Use: "buckets", Short: "Manage portfolio buckets", Args: cli.UsageArgs(cobra.NoArgs)}
	c.AddCommand(m.bucketList(), m.bucketCreate(), m.bucketUpdate(), m.bucketDelete())
	return c
}
func portfolioFlag(c *cobra.Command, v *string) {
	c.Flags().StringVar(v, "portfolio", "", "portfolio ID")
	_ = c.MarkFlagRequired("portfolio")
}
func (m Module) bucketList() *cobra.Command {
	var p string
	c := run("list", "List buckets", cobra.NoArgs, func(c *cobra.Command, _ []string) error {
		s, e := m.service(c)
		if e != nil {
			return e
		}
		v, e := s.BucketList(c.Context(), p)
		if e != nil {
			return e
		}
		return emitObject(c, map[string]any{"buckets": v}, "Buckets")
	})
	portfolioFlag(c, &p)
	return c
}

type bucketFlags struct {
	portfolio, name, kind, emoji string
	order                        int
}

func addBucketFlags(c *cobra.Command, f *bucketFlags, create bool) {
	portfolioFlag(c, &f.portfolio)
	c.Flags().StringVar(&f.name, "name", "", "bucket name")
	c.Flags().StringVar(&f.kind, "type", "", "bucket type")
	c.Flags().StringVar(&f.emoji, "emoji", "", "bucket emoji")
	c.Flags().IntVar(&f.order, "sort-order", 0, "sort order")
	if create {
		_ = c.MarkFlagRequired("name")
		_ = c.MarkFlagRequired("type")
	}
}
func validBucket(v string) bool {
	switch v {
	case "cash", "investments", "crypto", "real_estate", "vehicles", "other":
		return true
	}
	return false
}
func validAssetType(v string) bool {
	switch v {
	case "checking", "savings", "brokerage", "stock", "etf", "mutual_fund", "bond", "crypto_wallet", "crypto_exchange", "property", "vehicle", "loan", "mortgage", "credit_card", "other":
		return true
	}
	return false
}
func (m Module) bucketCreate() *cobra.Command {
	var f bucketFlags
	c := run("create", "Create a bucket", cobra.NoArgs, func(c *cobra.Command, _ []string) error {
		if !validBucket(f.kind) {
			return usage("type must be cash, investments, crypto, real_estate, vehicles or other")
		}
		b := daikuv1.BucketListRequest{Name: f.name, BucketType: daikuv1.BucketTypeEnum(f.kind)}
		if f.emoji != "" {
			b.Emoji = &f.emoji
		}
		if c.Flags().Changed("sort-order") {
			b.SortOrder = &f.order
		}
		s, e := m.service(c)
		if e != nil {
			return e
		}
		v, e := s.BucketCreate(c.Context(), f.portfolio, b)
		if e != nil {
			return e
		}
		return emitObject(c, v, "Bucket created")
	})
	addBucketFlags(c, &f, true)
	return c
}
func (m Module) bucketUpdate() *cobra.Command {
	var f bucketFlags
	c := run("update <id>", "Update a bucket", cobra.ExactArgs(1), func(c *cobra.Command, a []string) error {
		if !anyChanged(c, "name", "type", "emoji", "sort-order") {
			return usage("provide at least one field to update")
		}
		if f.kind != "" && !validBucket(f.kind) {
			return usage("invalid bucket type")
		}
		b := daikuv1.PatchedBucketListRequest{}
		if c.Flags().Changed("name") {
			b.Name = &f.name
		}
		if c.Flags().Changed("type") {
			v := daikuv1.BucketTypeEnum(f.kind)
			b.BucketType = &v
		}
		if c.Flags().Changed("emoji") {
			b.Emoji = &f.emoji
		}
		if c.Flags().Changed("sort-order") {
			b.SortOrder = &f.order
		}
		s, e := m.service(c)
		if e != nil {
			return e
		}
		v, e := s.BucketUpdate(c.Context(), f.portfolio, a[0], b)
		if e != nil {
			return e
		}
		return emitObject(c, v, "Bucket updated")
	})
	addBucketFlags(c, &f, false)
	return c
}
func (m Module) bucketDelete() *cobra.Command {
	var p string
	var yes bool
	c := run("delete <id>", "Delete a bucket", cobra.ExactArgs(1), func(c *cobra.Command, a []string) error {
		if e := confirm(c, yes, "Delete bucket "+a[0]+"."); e != nil {
			return e
		}
		s, e := m.service(c)
		if e != nil {
			return e
		}
		if e = s.BucketDelete(c.Context(), p, a[0]); e != nil {
			return e
		}
		return emit(c, map[string]any{"deleted": true, "id": a[0]}, "Bucket %s deleted.\n", a[0])
	})
	portfolioFlag(c, &p)
	c.Flags().BoolVar(&yes, "yes", false, "skip the interactive confirmation")
	return c
}

func (m Module) assets() *cobra.Command {
	c := &cobra.Command{Use: "assets", Short: "Manage assets, cashflows and value history", Args: cli.UsageArgs(cobra.NoArgs)}
	c.AddCommand(m.assetList(), m.assetCreate(), m.assetUpdate(), m.assetDelete(), m.cashflows(), m.history())
	return c
}
func bucketFlag(c *cobra.Command, v *string) {
	c.Flags().StringVar(v, "bucket", "", "bucket ID")
	_ = c.MarkFlagRequired("bucket")
}
func (m Module) assetList() *cobra.Command {
	var b string
	c := run("list", "List assets", cobra.NoArgs, func(c *cobra.Command, _ []string) error {
		s, e := m.service(c)
		if e != nil {
			return e
		}
		v, e := s.AssetList(c.Context(), b)
		if e != nil {
			return e
		}
		return emitObject(c, map[string]any{"assets": v}, "Assets")
	})
	bucketFlag(c, &b)
	return c
}

type assetFlags struct {
	bucket, name, kind, currency, value, quantity, price, ticker, institution, notes string
	liability, exclude                                                               bool
}

func addAssetFlags(c *cobra.Command, f *assetFlags, create bool) {
	bucketFlag(c, &f.bucket)
	c.Flags().StringVar(&f.name, "name", "", "asset name")
	c.Flags().StringVar(&f.kind, "type", "", "asset type")
	c.Flags().StringVar(&f.currency, "currency", "", "currency: UYU, USD or EUR")
	c.Flags().StringVar(&f.value, "current-value", "", "current value (sent verbatim to Daiku)")
	c.Flags().StringVar(&f.quantity, "quantity", "", "quantity")
	c.Flags().StringVar(&f.price, "price-per-unit", "", "price per unit")
	c.Flags().StringVar(&f.ticker, "ticker", "", "ticker symbol")
	c.Flags().StringVar(&f.institution, "institution", "", "institution ID")
	c.Flags().StringVar(&f.notes, "notes", "", "notes")
	c.Flags().BoolVar(&f.liability, "liability", false, "mark as liability")
	c.Flags().BoolVar(&f.exclude, "exclude-from-projections", false, "exclude from projections")
	if create {
		_ = c.MarkFlagRequired("name")
		_ = c.MarkFlagRequired("type")
	}
}
func (m Module) assetCreate() *cobra.Command {
	var f assetFlags
	c := run("create", "Create an asset", cobra.NoArgs, func(c *cobra.Command, _ []string) error {
		if !validAssetType(f.kind) {
			return usage("invalid asset type")
		}
		if f.currency != "" && !validCurrency(f.currency) {
			return usage("currency must be UYU, USD or EUR")
		}
		b := daikuv1.PublicAssetRequest{Name: f.name, AssetType: daikuv1.AssetTypeEnum(f.kind)}
		fillAssetCreate(c, &b, f)
		s, e := m.service(c)
		if e != nil {
			return e
		}
		v, e := s.AssetCreate(c.Context(), f.bucket, b)
		if e != nil {
			return e
		}
		return emitObject(c, v, "Asset created")
	})
	addAssetFlags(c, &f, true)
	return c
}
func fillAssetCreate(c *cobra.Command, b *daikuv1.PublicAssetRequest, f assetFlags) {
	if f.currency != "" {
		v := daikuv1.Currency3e8Enum(f.currency)
		b.Currency = &v
	}
	if c.Flags().Changed("current-value") {
		b.CurrentValue = &f.value
	}
	if c.Flags().Changed("quantity") {
		b.Quantity = &f.quantity
	}
	if c.Flags().Changed("price-per-unit") {
		b.PricePerUnit = &f.price
	}
	if c.Flags().Changed("ticker") {
		b.TickerSymbol = &f.ticker
	}
	if c.Flags().Changed("institution") {
		b.Institution = &f.institution
	}
	if c.Flags().Changed("notes") {
		b.Notes = &f.notes
	}
	if c.Flags().Changed("liability") {
		b.IsLiability = &f.liability
	}
	if c.Flags().Changed("exclude-from-projections") {
		b.ExcludeFromProjections = &f.exclude
	}
}
func (m Module) assetUpdate() *cobra.Command {
	var f assetFlags
	c := run("update <id>", "Update an asset", cobra.ExactArgs(1), func(c *cobra.Command, a []string) error {
		if !anyChanged(c, "name", "type", "currency", "current-value", "quantity", "price-per-unit", "ticker", "institution", "notes", "liability", "exclude-from-projections") {
			return usage("provide at least one field to update")
		}
		if f.currency != "" && !validCurrency(f.currency) {
			return usage("currency must be UYU, USD or EUR")
		}
		if c.Flags().Changed("type") && !validAssetType(f.kind) {
			return usage("invalid asset type")
		}
		b := daikuv1.PatchedPublicAssetRequest{}
		fillAssetPatch(c, &b, f)
		s, e := m.service(c)
		if e != nil {
			return e
		}
		v, e := s.AssetUpdate(c.Context(), f.bucket, a[0], b)
		if e != nil {
			return e
		}
		return emitObject(c, v, "Asset updated")
	})
	addAssetFlags(c, &f, false)
	return c
}
func fillAssetPatch(c *cobra.Command, b *daikuv1.PatchedPublicAssetRequest, f assetFlags) {
	if c.Flags().Changed("name") {
		b.Name = &f.name
	}
	if c.Flags().Changed("type") {
		v := daikuv1.AssetTypeEnum(f.kind)
		b.AssetType = &v
	}
	if c.Flags().Changed("currency") {
		v := daikuv1.Currency3e8Enum(f.currency)
		b.Currency = &v
	}
	if c.Flags().Changed("current-value") {
		b.CurrentValue = &f.value
	}
	if c.Flags().Changed("quantity") {
		b.Quantity = &f.quantity
	}
	if c.Flags().Changed("price-per-unit") {
		b.PricePerUnit = &f.price
	}
	if c.Flags().Changed("ticker") {
		b.TickerSymbol = &f.ticker
	}
	if c.Flags().Changed("institution") {
		b.Institution = &f.institution
	}
	if c.Flags().Changed("notes") {
		b.Notes = &f.notes
	}
	if c.Flags().Changed("liability") {
		b.IsLiability = &f.liability
	}
	if c.Flags().Changed("exclude-from-projections") {
		b.ExcludeFromProjections = &f.exclude
	}
}
func (m Module) assetDelete() *cobra.Command {
	var b string
	var yes bool
	c := run("delete <id>", "Delete an asset", cobra.ExactArgs(1), func(c *cobra.Command, a []string) error {
		if e := confirm(c, yes, "Delete asset "+a[0]+"."); e != nil {
			return e
		}
		s, e := m.service(c)
		if e != nil {
			return e
		}
		if e = s.AssetDelete(c.Context(), b, a[0]); e != nil {
			return e
		}
		return emit(c, map[string]any{"deleted": true, "id": a[0]}, "Asset %s deleted.\n", a[0])
	})
	bucketFlag(c, &b)
	c.Flags().BoolVar(&yes, "yes", false, "skip the interactive confirmation")
	return c
}

func (m Module) cashflows() *cobra.Command {
	c := &cobra.Command{Use: "cashflows", Short: "Manage asset cashflows", Args: cli.UsageArgs(cobra.NoArgs)}
	c.AddCommand(m.cashflowList(), m.cashflowCreate(), m.cashflowUpdate(), m.cashflowDelete())
	return c
}
func assetFlag(c *cobra.Command, v *string) {
	c.Flags().StringVar(v, "asset", "", "asset ID")
	_ = c.MarkFlagRequired("asset")
}
func (m Module) cashflowList() *cobra.Command {
	var a string
	c := run("list", "List cashflows, including transaction links", cobra.NoArgs, func(c *cobra.Command, _ []string) error {
		s, e := m.service(c)
		if e != nil {
			return e
		}
		v, e := s.CashflowList(c.Context(), a)
		if e != nil {
			return e
		}
		return emitObject(c, map[string]any{"cashflows": v}, "Cashflows")
	})
	assetFlag(c, &a)
	return c
}

type flowFlags struct{ asset, date, in, out, inCurrency, outCurrency, notes string }

func addFlowFlags(c *cobra.Command, f *flowFlags, create bool) {
	assetFlag(c, &f.asset)
	c.Flags().StringVar(&f.date, "date", "", "date (YYYY-MM-DD)")
	c.Flags().StringVar(&f.in, "cash-in", "", "cash in amount")
	c.Flags().StringVar(&f.out, "cash-out", "", "cash out amount")
	c.Flags().StringVar(&f.inCurrency, "cash-in-currency", "", "cash in currency")
	c.Flags().StringVar(&f.outCurrency, "cash-out-currency", "", "cash out currency")
	c.Flags().StringVar(&f.notes, "notes", "", "notes")
	if create {
		_ = c.MarkFlagRequired("date")
	}
}
func parseDate(v string) (openapi_types.Date, error) {
	t, e := time.Parse("2006-01-02", v)
	return openapi_types.Date{Time: t}, e
}
func flowCurrency(v string) (*daikuv1.AssetCashFlowRequest_CashInCurrency, error) {
	if !validCurrency(v) {
		return nil, usage("currency must be UYU, USD or EUR")
	}
	u := daikuv1.AssetCashFlowRequest_CashInCurrency{}
	e := u.FromCashInCurrencyEnum(daikuv1.CashInCurrencyEnum(v))
	return &u, e
}
func outFlowCurrency(v string) (*daikuv1.AssetCashFlowRequest_CashOutCurrency, error) {
	if !validCurrency(v) {
		return nil, usage("currency must be UYU, USD or EUR")
	}
	u := daikuv1.AssetCashFlowRequest_CashOutCurrency{}
	e := u.FromCashOutCurrencyEnum(daikuv1.CashOutCurrencyEnum(v))
	return &u, e
}
func (m Module) cashflowCreate() *cobra.Command {
	var f flowFlags
	c := run("create", "Create a cashflow", cobra.NoArgs, func(c *cobra.Command, _ []string) error {
		d, e := parseDate(f.date)
		if e != nil {
			return usage("date must use YYYY-MM-DD")
		}
		b := daikuv1.AssetCashFlowRequest{Date: d}
		if f.in != "" {
			b.CashIn = &f.in
		}
		if f.out != "" {
			b.CashOut = &f.out
		}
		if f.notes != "" {
			b.Notes = &f.notes
		}
		if f.inCurrency != "" {
			b.CashInCurrency, e = flowCurrency(f.inCurrency)
			if e != nil {
				return e
			}
		}
		if f.outCurrency != "" {
			b.CashOutCurrency, e = outFlowCurrency(f.outCurrency)
			if e != nil {
				return e
			}
		}
		s, e := m.service(c)
		if e != nil {
			return e
		}
		v, e := s.CashflowCreate(c.Context(), f.asset, b)
		if e != nil {
			return e
		}
		return emitObject(c, v, "Cashflow created")
	})
	addFlowFlags(c, &f, true)
	return c
}
func (m Module) cashflowUpdate() *cobra.Command {
	var f flowFlags
	c := run("update <id>", "Update a cashflow", cobra.ExactArgs(1), func(c *cobra.Command, a []string) error {
		if !anyChanged(c, "date", "cash-in", "cash-out", "cash-in-currency", "cash-out-currency", "notes") {
			return usage("provide at least one field to update")
		}
		b := daikuv1.PatchedAssetCashFlowRequest{}
		var e error
		if c.Flags().Changed("date") {
			d, x := parseDate(f.date)
			if x != nil {
				return usage("date must use YYYY-MM-DD")
			}
			b.Date = &d
		}
		if c.Flags().Changed("cash-in") {
			b.CashIn = &f.in
		}
		if c.Flags().Changed("cash-out") {
			b.CashOut = &f.out
		}
		if c.Flags().Changed("notes") {
			b.Notes = &f.notes
		}
		if c.Flags().Changed("cash-in-currency") {
			u, x := patchInCurrency(f.inCurrency)
			if x != nil {
				return x
			}
			b.CashInCurrency = u
		}
		if c.Flags().Changed("cash-out-currency") {
			u, x := patchOutCurrency(f.outCurrency)
			if x != nil {
				return x
			}
			b.CashOutCurrency = u
		}
		s, e := m.service(c)
		if e != nil {
			return e
		}
		v, e := s.CashflowUpdate(c.Context(), f.asset, a[0], b)
		if e != nil {
			return e
		}
		return emitObject(c, v, "Cashflow updated")
	})
	addFlowFlags(c, &f, false)
	return c
}
func patchInCurrency(v string) (*daikuv1.PatchedAssetCashFlowRequest_CashInCurrency, error) {
	if !validCurrency(v) {
		return nil, usage("currency must be UYU, USD or EUR")
	}
	u := daikuv1.PatchedAssetCashFlowRequest_CashInCurrency{}
	e := u.FromCashInCurrencyEnum(daikuv1.CashInCurrencyEnum(v))
	return &u, e
}
func patchOutCurrency(v string) (*daikuv1.PatchedAssetCashFlowRequest_CashOutCurrency, error) {
	if !validCurrency(v) {
		return nil, usage("currency must be UYU, USD or EUR")
	}
	u := daikuv1.PatchedAssetCashFlowRequest_CashOutCurrency{}
	e := u.FromCashOutCurrencyEnum(daikuv1.CashOutCurrencyEnum(v))
	return &u, e
}
func (m Module) cashflowDelete() *cobra.Command {
	var a string
	var yes bool
	c := run("delete <id>", "Delete a cashflow", cobra.ExactArgs(1), func(c *cobra.Command, v []string) error {
		if e := confirm(c, yes, "Delete cashflow "+v[0]+"."); e != nil {
			return e
		}
		s, e := m.service(c)
		if e != nil {
			return e
		}
		if e = s.CashflowDelete(c.Context(), a, v[0]); e != nil {
			return e
		}
		return emit(c, map[string]any{"deleted": true, "id": v[0]}, "Cashflow %s deleted.\n", v[0])
	})
	assetFlag(c, &a)
	c.Flags().BoolVar(&yes, "yes", false, "skip the interactive confirmation")
	return c
}

func (m Module) history() *cobra.Command {
	c := &cobra.Command{Use: "value-history", Short: "Manage asset value history", Args: cli.UsageArgs(cobra.NoArgs)}
	c.AddCommand(m.historyList(), m.historyCreate(), m.historyUpdate(), m.historyDelete())
	return c
}
func (m Module) historyList() *cobra.Command {
	var a string
	c := run("list", "List value history", cobra.NoArgs, func(c *cobra.Command, _ []string) error {
		s, e := m.service(c)
		if e != nil {
			return e
		}
		v, e := s.HistoryList(c.Context(), a)
		if e != nil {
			return e
		}
		return emitObject(c, map[string]any{"value_history": v}, "Value history")
	})
	assetFlag(c, &a)
	return c
}

type historyFlags struct{ asset, date, value, quantity, currency, notes string }

func addHistoryFlags(c *cobra.Command, f *historyFlags, create bool) {
	assetFlag(c, &f.asset)
	c.Flags().StringVar(&f.date, "date", "", "date (YYYY-MM-DD)")
	c.Flags().StringVar(&f.value, "value", "", "recorded value")
	c.Flags().StringVar(&f.quantity, "quantity", "", "recorded quantity")
	c.Flags().StringVar(&f.currency, "currency", "", "currency: UYU, USD or EUR")
	c.Flags().StringVar(&f.notes, "notes", "", "notes")
	if create {
		_ = c.MarkFlagRequired("date")
		_ = c.MarkFlagRequired("quantity")
	}
}
func (m Module) historyCreate() *cobra.Command {
	var f historyFlags
	c := run("create", "Create a value-history point", cobra.NoArgs, func(c *cobra.Command, _ []string) error {
		d, e := parseDate(f.date)
		if e != nil {
			return usage("date must use YYYY-MM-DD")
		}
		if f.currency != "" && !validCurrency(f.currency) {
			return usage("currency must be UYU, USD or EUR")
		}
		b := daikuv1.AssetValueHistoryRequest{Date: d, Quantity: &f.quantity}
		if f.value != "" {
			b.Value = &f.value
		}
		if f.currency != "" {
			v := daikuv1.Currency3e8Enum(f.currency)
			b.Currency = &v
		}
		if f.notes != "" {
			b.Notes = &f.notes
		}
		s, e := m.service(c)
		if e != nil {
			return e
		}
		v, e := s.HistoryCreate(c.Context(), f.asset, b)
		if e != nil {
			return e
		}
		return emitObject(c, v, "Value-history point created")
	})
	addHistoryFlags(c, &f, true)
	return c
}
func (m Module) historyUpdate() *cobra.Command {
	var f historyFlags
	c := run("update <id>", "Update a value-history point", cobra.ExactArgs(1), func(c *cobra.Command, a []string) error {
		if !anyChanged(c, "date", "value", "quantity", "currency", "notes") {
			return usage("provide at least one field to update")
		}
		if f.currency != "" && !validCurrency(f.currency) {
			return usage("currency must be UYU, USD or EUR")
		}
		b := daikuv1.PatchedAssetValueHistoryRequest{}
		if c.Flags().Changed("date") {
			d, e := parseDate(f.date)
			if e != nil {
				return usage("date must use YYYY-MM-DD")
			}
			b.Date = &d
		}
		if c.Flags().Changed("value") {
			b.Value = &f.value
		}
		if c.Flags().Changed("quantity") {
			b.Quantity = &f.quantity
		}
		if c.Flags().Changed("currency") {
			v := daikuv1.Currency3e8Enum(f.currency)
			b.Currency = &v
		}
		if c.Flags().Changed("notes") {
			b.Notes = &f.notes
		}
		s, e := m.service(c)
		if e != nil {
			return e
		}
		v, e := s.HistoryUpdate(c.Context(), f.asset, a[0], b)
		if e != nil {
			return e
		}
		return emitObject(c, v, "Value-history point updated")
	})
	addHistoryFlags(c, &f, false)
	return c
}
func (m Module) historyDelete() *cobra.Command {
	var a string
	var yes bool
	c := run("delete <id>", "Delete a value-history point", cobra.ExactArgs(1), func(c *cobra.Command, v []string) error {
		if e := confirm(c, yes, "Delete value-history point "+v[0]+"."); e != nil {
			return e
		}
		s, e := m.service(c)
		if e != nil {
			return e
		}
		if e = s.HistoryDelete(c.Context(), a, v[0]); e != nil {
			return e
		}
		return emit(c, map[string]any{"deleted": true, "id": v[0]}, "Value-history point %s deleted.\n", v[0])
	})
	assetFlag(c, &a)
	c.Flags().BoolVar(&yes, "yes", false, "skip the interactive confirmation")
	return c
}
func anyChanged(c *cobra.Command, names ...string) bool {
	for _, n := range names {
		if c.Flags().Changed(n) {
			return true
		}
	}
	return false
}
