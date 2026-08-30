package portfolios

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	daikuv1 "github.com/DaikuFi/daiku-cli/generated/daikuv1"
	"github.com/DaikuFi/daiku-cli/internal/cli"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

type fakeService struct {
	portfolios  []daikuv1.PortfolioList
	totals      *daikuv1.PortfolioTotals
	assets      []daikuv1.PublicAsset
	flows       []daikuv1.AssetCashFlow
	history     []daikuv1.AssetValueHistory
	lastAsset   daikuv1.PublicAssetRequest
	lastHistory daikuv1.AssetValueHistoryRequest
	lastPatch   map[string]any
}

func (f *fakeService) PortfolioList(context.Context) ([]daikuv1.PortfolioList, error) {
	return f.portfolios, nil
}
func (f *fakeService) PortfolioGet(context.Context, string) (*daikuv1.PublicPortfolio, error) {
	return &daikuv1.PublicPortfolio{}, nil
}
func (f *fakeService) PortfolioCreate(context.Context, daikuv1.PortfolioListRequest) (*daikuv1.PortfolioList, error) {
	return &daikuv1.PortfolioList{}, nil
}
func (f *fakeService) PortfolioUpdate(context.Context, string, map[string]any) (*daikuv1.PortfolioList, error) {
	return &daikuv1.PortfolioList{}, nil
}
func (f *fakeService) PortfolioDelete(context.Context, string) error { return nil }
func (f *fakeService) Totals(context.Context, string) (*daikuv1.PortfolioTotals, error) {
	return f.totals, nil
}
func (f *fakeService) Holdings(context.Context, string) (*daikuv1.PortfolioHoldings, error) {
	return &daikuv1.PortfolioHoldings{}, nil
}
func (f *fakeService) BucketList(context.Context, string) ([]daikuv1.BucketList, error) {
	return nil, nil
}
func (f *fakeService) BucketCreate(context.Context, string, daikuv1.BucketListRequest) (*daikuv1.BucketList, error) {
	return &daikuv1.BucketList{}, nil
}
func (f *fakeService) BucketUpdate(context.Context, string, string, map[string]any) (*daikuv1.BucketList, error) {
	return &daikuv1.BucketList{}, nil
}
func (f *fakeService) BucketDelete(context.Context, string, string) error { return nil }
func (f *fakeService) AssetList(context.Context, string) ([]daikuv1.PublicAsset, error) {
	return f.assets, nil
}
func (f *fakeService) AssetCreate(_ context.Context, _ string, v daikuv1.PublicAssetRequest) (*daikuv1.PublicAsset, error) {
	f.lastAsset = v
	return &daikuv1.PublicAsset{Name: v.Name, AssetType: v.AssetType, IsLiability: v.IsLiability, Currency: v.Currency}, nil
}
func (f *fakeService) AssetUpdate(_ context.Context, _, _ string, patch map[string]any) (*daikuv1.PublicAsset, error) {
	f.lastPatch = patch
	return &daikuv1.PublicAsset{}, nil
}
func (f *fakeService) AssetDelete(context.Context, string, string) error { return nil }
func (f *fakeService) CashflowList(context.Context, string) ([]daikuv1.AssetCashFlow, error) {
	return f.flows, nil
}
func (f *fakeService) CashflowCreate(context.Context, string, daikuv1.AssetCashFlowRequest) (*daikuv1.AssetCashFlow, error) {
	return &daikuv1.AssetCashFlow{}, nil
}
func (f *fakeService) CashflowUpdate(_ context.Context, _, _ string, patch map[string]any) (*daikuv1.AssetCashFlow, error) {
	f.lastPatch = patch
	return &daikuv1.AssetCashFlow{}, nil
}
func (f *fakeService) CashflowDelete(context.Context, string, string) error { return nil }
func (f *fakeService) HistoryList(context.Context, string) ([]daikuv1.AssetValueHistory, error) {
	return f.history, nil
}

func (f *fakeService) HistoryCreate(_ context.Context, _ string, request daikuv1.AssetValueHistoryRequest) (*daikuv1.AssetValueHistory, error) {
	f.lastHistory = request
	return &daikuv1.AssetValueHistory{}, nil
}
func (f *fakeService) HistoryUpdate(_ context.Context, _, _ string, patch map[string]any) (*daikuv1.AssetValueHistory, error) {
	f.lastPatch = patch
	return &daikuv1.AssetValueHistory{}, nil
}
func (f *fakeService) HistoryDelete(context.Context, string, string) error { return nil }

func execute(t *testing.T, f *fakeService, args ...string) (int, string, string) {
	t.Helper()
	var out, errOut bytes.Buffer
	m := New(func(context.Context) (Service, error) { return f, nil })
	app := cli.New(cli.WithIO(strings.NewReader(""), &out, &errOut), cli.WithModule(m), cli.WithEnvironment(func(string) (string, bool) { return "", false }))
	code := app.Run(args)
	return code, out.String(), errOut.String()
}

func TestTotalsPreserveServerLiabilityMathAndCurrency(t *testing.T) {
	f := &fakeService{totals: &daikuv1.PortfolioTotals{DisplayCurrency: "USD", TotalAssets: "100.00", TotalLiabilities: "35.00", NetWorth: "65.00"}}
	code, out, stderr := execute(t, f, "portfolios", "totals", "prt_1", "--json")
	if code != 0 || stderr != "" {
		t.Fatalf("code=%d stderr=%s", code, stderr)
	}
	for _, want := range []string{`"display_currency":"USD"`, `"total_assets":"100.00"`, `"total_liabilities":"35.00"`, `"net_worth":"65.00"`} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %s in %s", want, out)
		}
	}
}

func TestTotalsHumanOutputIsSpanishAndTabular(t *testing.T) {
	f := &fakeService{totals: &daikuv1.PortfolioTotals{DisplayCurrency: "USD", TotalAssets: "100.00", TotalLiabilities: "35.00", NetWorth: "65.00"}}
	code, out, stderr := execute(t, f, "portfolios", "totals", "prt_1", "--language", "es")
	if code != 0 || stderr != "" {
		t.Fatalf("code=%d stderr=%s", code, stderr)
	}
	if !strings.Contains(out, "Totales del portafolio (calculados por Daiku)") || !strings.Contains(out, "PASIVOS TOTALES") || !strings.Contains(out, "35.00") {
		t.Fatalf("output=%s", out)
	}
}

func TestSpanishHeadersAndErrorsAreLocalized(t *testing.T) {
	f := &fakeService{totals: &daikuv1.PortfolioTotals{DisplayCurrency: "USD", TotalAssets: "1", TotalLiabilities: "0", NetWorth: "1"}}
	code, out, _ := execute(t, f, "portfolios", "totals", "prt_1", "--language", "es")
	if code != 0 || !strings.Contains(out, "PATRIMONIO") || !strings.Contains(out, "PASIVOS TOTALES") {
		t.Fatalf("output=%s", out)
	}
	code, _, stderr := execute(t, &fakeService{}, "assets", "create", "--bucket", "bkt_1", "--name", "x", "--type", "other", "--currency", "BTC", "--language", "es")
	if code != int(cli.ExitUsage) || !strings.Contains(stderr, "moneda no admitida") {
		t.Fatalf("stderr=%s", stderr)
	}
}

func TestAssetCreatePassesLiabilityAndCurrencyWithoutRecalculation(t *testing.T) {
	f := &fakeService{}
	code, _, stderr := execute(t, f, "assets", "create", "--bucket", "bkt_1", "--name", "Loan", "--type", "loan", "--currency", "UYU", "--current-value", "123.45", "--liability", "--json")
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr)
	}
	if f.lastAsset.IsLiability == nil || !*f.lastAsset.IsLiability || f.lastAsset.CurrentValue == nil || *f.lastAsset.CurrentValue != "123.45" || f.lastAsset.Currency == nil || *f.lastAsset.Currency != "UYU" {
		t.Fatalf("request changed: %#v", f.lastAsset)
	}
}

func TestAssetPatchClearFlagsProduceExplicitNullAndOmitOthers(t *testing.T) {
	f := &fakeService{}
	code, _, stderr := execute(t, f, "assets", "update", "ast_1", "--bucket", "bkt_1", "--clear-quantity", "--clear-price-per-unit", "--clear-ticker", "--clear-last-price-update", "--json")
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr)
	}
	if value, ok := f.lastPatch["quantity"]; !ok || value != nil {
		t.Fatalf("quantity not explicit null: %#v", f.lastPatch)
	}
	if value, ok := f.lastPatch["price_per_unit"]; !ok || value != nil {
		t.Fatalf("price not explicit null: %#v", f.lastPatch)
	}
	if _, ok := f.lastPatch["ticker_symbol"]; ok {
		if f.lastPatch["ticker_symbol"] != nil {
			t.Fatalf("ticker not cleared: %#v", f.lastPatch)
		}
	}
	if value, ok := f.lastPatch["last_price_update"]; !ok || value != nil {
		t.Fatalf("last price not cleared: %#v", f.lastPatch)
	}
}

func TestAssetPatchOmitsNullableFieldsUnlessChanged(t *testing.T) {
	f := &fakeService{}
	code, _, _ := execute(t, f, "assets", "update", "ast_1", "--bucket", "bkt_1", "--name", "Renamed", "--json")
	if code != 0 {
		t.Fatal(code)
	}
	for _, key := range []string{"quantity", "price_per_unit", "ticker_symbol", "last_price_update"} {
		if _, ok := f.lastPatch[key]; ok {
			t.Fatalf("%s should be omitted: %#v", key, f.lastPatch)
		}
	}
}

func TestCashflowPatchOmitNullAndExtraCurrency(t *testing.T) {
	f := &fakeService{}
	code, _, stderr := execute(t, f, "assets", "cashflows", "update", "cf_1", "--asset", "ast_1", "--cash-in-currency", "BRL", "--clear-cash-out", "--json")
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr)
	}
	if f.lastPatch["cash_in_currency"] != "BRL" {
		t.Fatalf("currency missing: %#v", f.lastPatch)
	}
	if value, ok := f.lastPatch["cash_out"]; !ok || value != nil {
		t.Fatalf("clear missing: %#v", f.lastPatch)
	}
	if _, ok := f.lastPatch["cash_in"]; ok {
		t.Fatalf("cash_in should be omitted: %#v", f.lastPatch)
	}
}

func TestHistoryQuantityOmittedOrExplicitlyCleared(t *testing.T) {
	f := &fakeService{}
	code, _, _ := execute(t, f, "assets", "value-history", "update", "vh_1", "--asset", "ast_1", "--notes", "x", "--json")
	if code != 0 {
		t.Fatal(code)
	}
	if _, ok := f.lastPatch["quantity"]; ok {
		t.Fatalf("quantity should be omitted: %#v", f.lastPatch)
	}
	code, _, _ = execute(t, f, "assets", "value-history", "update", "vh_1", "--asset", "ast_1", "--clear-quantity", "--json")
	if code != 0 {
		t.Fatal(code)
	}
	if value, ok := f.lastPatch["quantity"]; !ok || value != nil {
		t.Fatalf("quantity should be null: %#v", f.lastPatch)
	}
}

func TestHistoryCreateQuantityIsOptionalAndPresenceAware(t *testing.T) {
	f := &fakeService{}
	code, _, stderr := execute(t, f, "assets", "value-history", "create", "--asset", "ast_1", "--date", "2026-08-30", "--value", "25", "--json")
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr)
	}
	if f.lastHistory.Quantity != nil {
		t.Fatalf("quantity should be omitted: %#v", f.lastHistory)
	}
	if f.lastHistory.Value == nil || *f.lastHistory.Value != "25" {
		t.Fatalf("value missing: %#v", f.lastHistory)
	}

	code, _, stderr = execute(t, f, "assets", "value-history", "create", "--asset", "ast_1", "--date", "2026-08-30", "--quantity", "2.5", "--json")
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr)
	}
	if f.lastHistory.Quantity == nil || *f.lastHistory.Quantity != "2.5" {
		t.Fatalf("quantity missing: %#v", f.lastHistory)
	}
}

func TestHistoryRejectsClearValueBecauseValueIsNotNullable(t *testing.T) {
	f := &fakeService{}
	code, _, stderr := execute(t, f, "assets", "value-history", "update", "vh_1", "--asset", "ast_1", "--clear-value", "--json")
	if code != int(cli.ExitUsage) || !strings.Contains(stderr, `"code":"usage_error"`) || !strings.Contains(stderr, "unknown flag: --clear-value") {
		t.Fatalf("code=%d stderr=%s", code, stderr)
	}
	if f.lastPatch != nil {
		t.Fatalf("service should not be called: %#v", f.lastPatch)
	}
}

func TestInvalidCurrencyRejected(t *testing.T) {
	code, _, stderr := execute(t, &fakeService{}, "assets", "create", "--bucket", "bkt_1", "--name", "x", "--type", "other", "--currency", "BTC", "--json")
	if code != int(cli.ExitUsage) || !strings.Contains(stderr, "unsupported currency") {
		t.Fatalf("code=%d stderr=%s", code, stderr)
	}
}

func TestCurrencyFlagsRejectExplicitEmptyValues(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{name: "portfolio create", args: []string{"portfolios", "create", "--name", "Main", "--display-currency", "", "--json"}},
		{name: "portfolio", args: []string{"portfolios", "update", "prt_1", "--display-currency", "", "--json"}},
		{name: "asset create", args: []string{"assets", "create", "--bucket", "bkt_1", "--name", "Cash", "--type", "other", "--currency", "", "--json"}},
		{name: "asset", args: []string{"assets", "update", "ast_1", "--bucket", "bkt_1", "--currency", "", "--json"}},
		{name: "value history create", args: []string{"assets", "value-history", "create", "--asset", "ast_1", "--date", "2026-08-30", "--currency", "", "--json"}},
		{name: "value history", args: []string{"assets", "value-history", "update", "vh_1", "--asset", "ast_1", "--currency", "", "--json"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			code, _, stderr := execute(t, &fakeService{}, test.args...)
			if code != int(cli.ExitUsage) || !strings.Contains(stderr, "unsupported currency") {
				t.Fatalf("code=%d stderr=%s", code, stderr)
			}
		})
	}
}

func TestSetAndClearFlagsConflict(t *testing.T) {
	code, _, stderr := execute(t, &fakeService{}, "assets", "cashflows", "update", "cf_1", "--asset", "ast_1", "--cash-in", "2", "--clear-cash-in", "--json")
	if code != int(cli.ExitUsage) || !strings.Contains(stderr, "cannot set and clear cash-in together") {
		t.Fatalf("code=%d stderr=%s", code, stderr)
	}
}

func TestSetAndClearErrorIsFullySpanish(t *testing.T) {
	code, _, stderr := execute(t, &fakeService{}, "assets", "cashflows", "update", "cf_1", "--asset", "ast_1", "--cash-in", "2", "--clear-cash-in", "--language", "es")
	if code != int(cli.ExitUsage) || !strings.Contains(stderr, "no se puede establecer y borrar entrada a la vez") || strings.Contains(stderr, "together") {
		t.Fatalf("code=%d stderr=%s", code, stderr)
	}
}

func TestContractHeadersAreSpanish(t *testing.T) {
	now := time.Date(2026, 8, 30, 12, 0, 0, 0, time.UTC)
	currency := daikuv1.Currency3e8Enum("BRL")
	value := "10"
	excluded := true
	institution := "ins_1"
	f := &fakeService{assets: []daikuv1.PublicAsset{{Name: "Casa", AssetType: "property", Currency: &currency, CurrentValue: &value, ExcludeFromProjections: &excluded, Institution: &institution, LastPriceUpdate: &now, LinkedAccount: &daikuv1.LinkedAccount{Id: "acc_1", Name: "Cuenta"}}}}
	code, out, stderr := execute(t, f, "assets", "list", "--bucket", "bkt_1", "--language", "es")
	if code != 0 {
		t.Fatalf("stderr=%s", stderr)
	}
	for _, want := range []string{"MONEDA", "VALOR ACTUAL", "EXCLUIDO DE PROYECCIONES", "INSTITUCIÓN", "ÚLTIMO PRECIO", "CUENTA VINCULADA", "TIPO DE ACTIVO"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in %s", want, out)
		}
	}
}

func TestRepresentativeCRUDCommands(t *testing.T) {
	f := &fakeService{}
	commands := [][]string{{"portfolios", "create", "--name", "Main", "--json"}, {"portfolios", "update", "prt_1", "--name", "New", "--json"}, {"portfolios", "delete", "prt_1", "--yes", "--json"}, {"portfolios", "buckets", "create", "--portfolio", "prt_1", "--name", "Cash", "--type", "cash", "--json"}, {"assets", "cashflows", "create", "--asset", "ast_1", "--date", "2026-08-30", "--cash-in", "10", "--cash-in-currency", "BRL", "--json"}, {"assets", "value-history", "create", "--asset", "ast_1", "--date", "2026-08-30", "--quantity", "1", "--currency", "GBP", "--json"}}
	for _, args := range commands {
		code, _, stderr := execute(t, f, args...)
		if code != 0 {
			t.Fatalf("%v code=%d stderr=%s", args, code, stderr)
		}
	}
}

func TestAssetListPreservesLinkedAccount(t *testing.T) {
	id, name := "acc_1", "Checking"
	f := &fakeService{assets: []daikuv1.PublicAsset{{Name: "Cash", AssetType: "checking", LinkedAccount: &daikuv1.LinkedAccount{Id: id, Name: name}}}}
	code, out, _ := execute(t, f, "assets", "list", "--bucket", "bkt_1", "--json")
	if code != 0 || !strings.Contains(out, `"linked_account":{"id":"acc_1","name":"Checking"}`) {
		t.Fatalf("output=%s", out)
	}
}

func TestCashflowListPreservesTransactionLinks(t *testing.T) {
	id, expense := "lnk_1", "exp_1"
	f := &fakeService{flows: []daikuv1.AssetCashFlow{{Date: openapi_types.Date{Time: time.Date(2026, 8, 30, 0, 0, 0, 0, time.UTC)}, TransactionLinks: &daikuv1.CashFlowTransactionLinks{CashIn: &daikuv1.CashFlowTransactionLink{Id: id, Side: "cash_in", Visibility: "visible", Transaction: &daikuv1.CashFlowLinkedTransaction{Id: expense, Amount: "10.00", Currency: "USD", Description: "Deposit", ExpenseDate: openapi_types.Date{Time: time.Date(2026, 8, 30, 0, 0, 0, 0, time.UTC)}, Household: "hh_1"}}}}}}
	code, out, _ := execute(t, f, "assets", "cashflows", "list", "--asset", "ast_1", "--json")
	if code != 0 || !strings.Contains(out, `"transaction":{"account_name":null,"amount":"10.00","currency":"USD","description":"Deposit","expense_date":"2026-08-30","household":"hh_1","id":"exp_1"`) {
		t.Fatalf("output=%s", out)
	}
}

func TestEmptyValueHistoryHasStableEmptyArray(t *testing.T) {
	code, out, _ := execute(t, &fakeService{history: []daikuv1.AssetValueHistory{}}, "assets", "value-history", "list", "--asset", "ast_1", "--json")
	if code != 0 || !strings.Contains(out, `"value_history":[]`) {
		t.Fatalf("output=%s", out)
	}
}

func TestLargeValueHistoryRemainsComplete(t *testing.T) {
	items := make([]daikuv1.AssetValueHistory, 1000)
	for index := range items {
		id := fmt.Sprintf("vh_%04d", index)
		items[index] = daikuv1.AssetValueHistory{Id: &id, Date: openapi_types.Date{Time: time.Date(2026, 8, 30, 0, 0, 0, 0, time.UTC)}, Quantity: stringPtr("1")}
	}
	code, out, _ := execute(t, &fakeService{history: items}, "assets", "value-history", "list", "--asset", "ast_1", "--json")
	if code != 0 || !strings.Contains(out, "vh_0999") {
		t.Fatalf("incomplete output: code=%d len=%d", code, len(out))
	}
}

func stringPtr(value string) *string { return &value }

func TestCrossUserNotFoundIsSafe(t *testing.T) {
	err := apiError(404, []byte(`{"code":"not_found","message":"not found"}`))
	cliErr, ok := err.(*cli.Error)
	if !ok || cliErr.ExitCode != cli.ExitNotFound || strings.Contains(strings.ToLower(cliErr.Message), "owner") {
		t.Fatalf("unsafe mapping: %#v", err)
	}
}

func TestDestructiveCommandRequiresConfirmationWhenPiped(t *testing.T) {
	code, _, stderr := execute(t, &fakeService{}, "assets", "delete", "ast_1", "--bucket", "bkt_1", "--json")
	if code != int(cli.ExitUsage) || !strings.Contains(stderr, "confirmation_required") {
		t.Fatalf("code=%d stderr=%s", code, stderr)
	}
}
