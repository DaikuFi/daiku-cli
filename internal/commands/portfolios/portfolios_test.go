package portfolios

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	daikuv1 "github.com/DaikuFi/daiku-cli/generated/daikuv1"
	"github.com/DaikuFi/daiku-cli/internal/cli"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

type fakeService struct {
	portfolios []daikuv1.PortfolioList
	totals     *daikuv1.PortfolioTotals
	assets     []daikuv1.PublicAsset
	flows      []daikuv1.AssetCashFlow
	history    []daikuv1.AssetValueHistory
	lastAsset  daikuv1.PublicAssetRequest
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
func (f *fakeService) PortfolioUpdate(context.Context, string, daikuv1.PatchedPortfolioListRequest) (*daikuv1.PortfolioList, error) {
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
func (f *fakeService) BucketUpdate(context.Context, string, string, daikuv1.PatchedBucketListRequest) (*daikuv1.BucketList, error) {
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
func (f *fakeService) AssetUpdate(context.Context, string, string, daikuv1.PatchedPublicAssetRequest) (*daikuv1.PublicAsset, error) {
	return &daikuv1.PublicAsset{}, nil
}
func (f *fakeService) AssetDelete(context.Context, string, string) error { return nil }
func (f *fakeService) CashflowList(context.Context, string) ([]daikuv1.AssetCashFlow, error) {
	return f.flows, nil
}
func (f *fakeService) CashflowCreate(context.Context, string, daikuv1.AssetCashFlowRequest) (*daikuv1.AssetCashFlow, error) {
	return &daikuv1.AssetCashFlow{}, nil
}
func (f *fakeService) CashflowUpdate(context.Context, string, string, daikuv1.PatchedAssetCashFlowRequest) (*daikuv1.AssetCashFlow, error) {
	return &daikuv1.AssetCashFlow{}, nil
}
func (f *fakeService) CashflowDelete(context.Context, string, string) error { return nil }
func (f *fakeService) HistoryList(context.Context, string) ([]daikuv1.AssetValueHistory, error) {
	return f.history, nil
}
func (f *fakeService) HistoryCreate(context.Context, string, daikuv1.AssetValueHistoryRequest) (*daikuv1.AssetValueHistory, error) {
	return &daikuv1.AssetValueHistory{}, nil
}
func (f *fakeService) HistoryUpdate(context.Context, string, string, daikuv1.PatchedAssetValueHistoryRequest) (*daikuv1.AssetValueHistory, error) {
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
	if !strings.Contains(out, "Totales del portafolio (calculados por Daiku)") || !strings.Contains(out, "TOTAL LIABILITIES") || !strings.Contains(out, "35.00") {
		t.Fatalf("output=%s", out)
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
