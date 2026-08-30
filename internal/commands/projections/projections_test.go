package projections

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	daikuv1 "github.com/DaikuFi/daiku-cli/generated/daikuv1"
	"github.com/DaikuFi/daiku-cli/internal/cli"
	"github.com/oapi-codegen/runtime/types"
)

type fakeAPI struct {
	scenarios []daikuv1.ProjectionScenario
	rules     []daikuv1.ProjectionRule
	series    *daikuv1.NetWorthSeries
	rates     []daikuv1.ExchangeRate
	rateDate  *types.Date
	created   *daikuv1.ProjectionRuleRequest
	deleted   string
	err       error
}

func (f *fakeAPI) ScenarioList(context.Context, string) ([]daikuv1.ProjectionScenario, error) {
	return f.scenarios, f.err
}
func (f *fakeAPI) ScenarioCreate(_ context.Context, _ string, b daikuv1.ProjectionScenarioRequest) (*daikuv1.ProjectionScenario, error) {
	return &daikuv1.ProjectionScenario{Name: b.Name}, f.err
}
func (f *fakeAPI) ScenarioUpdate(context.Context, string, string, daikuv1.PatchedProjectionScenarioRequest) (*daikuv1.ProjectionScenario, error) {
	return &daikuv1.ProjectionScenario{Name: "updated"}, f.err
}
func (f *fakeAPI) ScenarioDelete(_ context.Context, _ string, id string) error {
	f.deleted = id
	return f.err
}
func (f *fakeAPI) Calculate(context.Context, string, string) (*daikuv1.ProjectionResult, error) {
	return &daikuv1.ProjectionResult{ScenarioId: "scn_1", MonthlySnapshots: []daikuv1.ProjectionSnapshot{}}, f.err
}
func (f *fakeAPI) Retirement(context.Context, string, string) (*daikuv1.RetirementResult, error) {
	return &daikuv1.RetirementResult{CanRetire: true}, f.err
}
func (f *fakeAPI) RuleList(context.Context, string) ([]daikuv1.ProjectionRule, error) {
	return f.rules, f.err
}
func (f *fakeAPI) RuleCreate(_ context.Context, _ string, b daikuv1.ProjectionRuleRequest) (*daikuv1.ProjectionRule, error) {
	f.created = &b
	return &daikuv1.ProjectionRule{Category: b.Category, Config: daikuv1.ProjectionRuleConfig{}, RuleType: b.RuleType}, f.err
}
func (f *fakeAPI) RuleUpdate(context.Context, string, string, daikuv1.PatchedProjectionRuleRequest) (*daikuv1.ProjectionRule, error) {
	return &daikuv1.ProjectionRule{Category: daikuv1.ProjectionRuleCategoryEnumIncome, Config: daikuv1.ProjectionRuleConfig{}, RuleType: "salary"}, f.err
}
func (f *fakeAPI) RuleDelete(_ context.Context, _ string, id string) error {
	f.deleted = id
	return f.err
}
func (f *fakeAPI) NetWorth(context.Context) (*daikuv1.NetWorthSeries, error) {
	if f.series == nil {
		return &daikuv1.NetWorthSeries{Series: []daikuv1.NetWorthPoint{}}, f.err
	}
	return f.series, f.err
}
func (f *fakeAPI) CurrencyExposure(context.Context) (*daikuv1.CurrencyExposure, error) {
	return &daikuv1.CurrencyExposure{ByCurrency: []daikuv1.CurrencyExposureItem{}}, f.err
}
func (f *fakeAPI) Rates(_ context.Context, p *daikuv1.DaikuExchangeRatesGetParams) ([]daikuv1.ExchangeRate, error) {
	f.rateDate = p.Date
	return f.rates, f.err
}

func run(t *testing.T, api API, terminal bool, args ...string) (int, string, string) {
	t.Helper()
	out, errOut := &bytes.Buffer{}, &bytes.Buffer{}
	app := cli.New(
		cli.WithIO(strings.NewReader(""), out, errOut),
		cli.WithTerminalDetector(func(io.Writer) bool { return terminal }),
		cli.WithInteractiveDetector(func(io.Reader, io.Writer) bool { return false }),
		cli.WithTerminalWidthDetector(func(io.Writer) int { return 120 }),
		cli.WithModule(Module{Factory: func(context.Context) (API, error) { return api, nil }}),
	)
	return app.Run(args), out.String(), errOut.String()
}

func TestHistoricalRateDateIsDelegatedToServer(t *testing.T) {
	resolved := types.Date{Time: time.Date(2026, 8, 28, 0, 0, 0, 0, time.UTC)}
	api := &fakeAPI{rates: []daikuv1.ExchangeRate{{Date: &resolved, FromCurrency: "USD", ToCurrency: "UYU", Rate: "40.125000"}}}
	code, out, _ := run(t, api, false, "exchange-rates", "--date", "2026-08-30", "--json")
	if code != 0 || api.rateDate == nil || api.rateDate.String() != "2026-08-30" {
		t.Fatalf("code=%d requested=%v", code, api.rateDate)
	}
	if !strings.Contains(out, `"date":"2026-08-28"`) || !strings.Contains(out, `"rate":"40.125000"`) {
		t.Fatalf("server response changed: %s", out)
	}
}

func TestRateRejectsInvalidDateBeforeAPI(t *testing.T) {
	api := &fakeAPI{}
	code, _, errOut := run(t, api, false, "exchange-rates", "--date", "30/08/2026", "--json")
	if code != int(cli.ExitUsage) || api.rateDate != nil || !strings.Contains(errOut, "YYYY-MM-DD") {
		t.Fatalf("code=%d date=%v stderr=%q", code, api.rateDate, errOut)
	}
}

func TestInvalidScenarioAndMissingPortfolioAreTypedUsageErrors(t *testing.T) {
	api := &fakeAPI{}
	for _, args := range [][]string{
		{"projections", "calculate", "--portfolio", "prt_1", "--json"},
		{"projections", "calculate", "--scenario", "scn_1", "--json"},
	} {
		code, _, errOut := run(t, api, false, args...)
		if code != int(cli.ExitUsage) || !strings.Contains(errOut, `"code":"usage_error"`) {
			t.Fatalf("args=%v code=%d stderr=%q", args, code, errOut)
		}
	}
}

func TestRuleConfigIsForwardedWithoutLocalFinancialLogic(t *testing.T) {
	api := &fakeAPI{}
	config := `{"amount":"123.45","currency":"USD","frequency":"monthly","start_date":"2026-08-30"}`
	code, _, errOut := run(t, api, false, "projections", "rules", "create", "--scenario", "scn_1", "--category", "income", "--type", "salary", "--config", config, "--json")
	if code != 0 {
		t.Fatalf("code=%d stderr=%q", code, errOut)
	}
	if api.created == nil || api.created.Config.Amount == nil || *api.created.Config.Amount != "123.45" || api.created.Config.Currency == nil || string(*api.created.Config.Currency) != "USD" {
		t.Fatalf("request=%+v", api.created)
	}
}

func TestDestructiveCommandsRequireConfirmation(t *testing.T) {
	api := &fakeAPI{}
	code, _, errOut := run(t, api, false, "projections", "scenarios", "delete", "scn_1", "--portfolio", "prt_1", "--json")
	if code != int(cli.ExitUsage) || api.deleted != "" || !strings.Contains(errOut, "confirmation_required") {
		t.Fatalf("code=%d deleted=%q stderr=%q", code, api.deleted, errOut)
	}
}

func TestEmptySeriesHasStableJSONAndHumanEmptyState(t *testing.T) {
	api := &fakeAPI{series: &daikuv1.NetWorthSeries{Currency: "UYU", Series: []daikuv1.NetWorthPoint{}}}
	code, jsonOut, _ := run(t, api, false, "reports", "net-worth", "--json")
	if code != 0 || jsonOut != "{\"ok\":true,\"data\":{\"currency\":\"UYU\",\"series\":[]}}\n" {
		t.Fatalf("code=%d json=%q", code, jsonOut)
	}
	code, human, _ := run(t, api, true, "reports", "net-worth", "--language", "es")
	if code != 0 || strings.TrimSpace(human) != "No hay resultados." {
		t.Fatalf("code=%d human=%q", code, human)
	}
}

func TestLargeSeriesPreservesEveryBackendPointInJSON(t *testing.T) {
	points := make([]daikuv1.NetWorthPoint, 2500)
	for i := range points {
		points[i] = daikuv1.NetWorthPoint{Date: types.Date{Time: time.Date(2000+i/12, time.Month(i%12+1), 1, 0, 0, 0, 0, time.UTC)}, NetWorth: fmt.Sprintf("%d.00", i), Assets: "0.00", Liabilities: "0.00"}
	}
	api := &fakeAPI{series: &daikuv1.NetWorthSeries{Currency: "USD", Series: points}}
	code, out, _ := run(t, api, false, "reports", "net-worth", "--json")
	if code != 0 || strings.Count(out, `"net_worth"`) != len(points) || !strings.Contains(out, `"net_worth":"2499.00"`) {
		t.Fatalf("code=%d points=%d bytes=%d", code, strings.Count(out, `"net_worth"`), len(out))
	}
}

func TestTTYAndJSONCarrySameBackendValues(t *testing.T) {
	date := types.Date{Time: time.Date(2026, 8, 29, 0, 0, 0, 0, time.UTC)}
	api := &fakeAPI{series: &daikuv1.NetWorthSeries{Currency: "EUR", Series: []daikuv1.NetWorthPoint{{Date: date, NetWorth: "99.10", Assets: "120.20", Liabilities: "21.10"}}}}
	_, jsonOut, _ := run(t, api, false, "reports", "net-worth", "--json")
	_, humanOut, _ := run(t, api, true, "reports", "net-worth", "--language", "en")
	for _, value := range []string{"2026-08-29", "99.10", "120.20", "21.10", "EUR"} {
		if !strings.Contains(jsonOut, value) || !strings.Contains(humanOut, value) {
			t.Fatalf("value %q missing; json=%q human=%q", value, jsonOut, humanOut)
		}
	}
}
