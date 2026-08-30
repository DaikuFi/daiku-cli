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
	"testing"
	"time"

	daikuv1 "github.com/DaikuFi/daiku-cli/generated/daikuv1"
	"github.com/DaikuFi/daiku-cli/internal/cli"
	"github.com/oapi-codegen/runtime/types"
)

type fakeAPI struct {
	scenarios       []daikuv1.ProjectionScenario
	rules           []daikuv1.ProjectionRule
	series          *daikuv1.NetWorthSeries
	rates           []daikuv1.ExchangeRate
	rateDate        *types.Date
	created         *daikuv1.ProjectionRuleRequest
	updatedScenario *scenarioPatch
	updatedRule     *daikuv1.PatchedProjectionRuleRequest
	deleted         string
	err             error
}

func (f *fakeAPI) ScenarioList(context.Context, string) ([]daikuv1.ProjectionScenario, error) {
	return f.scenarios, f.err
}
func (f *fakeAPI) ScenarioCreate(_ context.Context, _ string, b daikuv1.ProjectionScenarioRequest) (*daikuv1.ProjectionScenario, error) {
	return &daikuv1.ProjectionScenario{Name: b.Name}, f.err
}
func (f *fakeAPI) ScenarioUpdate(_ context.Context, _, _ string, body scenarioPatch) (*daikuv1.ProjectionScenario, error) {
	f.updatedScenario = &body
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
func (f *fakeAPI) RuleUpdate(_ context.Context, _, _ string, body daikuv1.PatchedProjectionRuleRequest) (*daikuv1.ProjectionRule, error) {
	f.updatedRule = &body
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

func TestRuleConfigRejectsFieldsOutsidePinnedContract(t *testing.T) {
	api := &fakeAPI{}
	code, _, errOut := run(t, api, false, "projections", "rules", "create", "--scenario", "scn_1", "--category", "income", "--type", "salary", "--config", `{"amount":"1","typo_currency":"USD"}`, "--json")
	if code != int(cli.ExitUsage) || api.created != nil || !strings.Contains(errOut, "valid JSON object") {
		t.Fatalf("code=%d request=%+v stderr=%q", code, api.created, errOut)
	}
}

func TestRuleConfigRejectsInvalidContractEnums(t *testing.T) {
	for _, config := range []string{
		`{"currency":"BTC"}`,
		`{"frequency":"weekly"}`,
		`{"target_bucket_type":"banana"}`,
	} {
		api := &fakeAPI{}
		code, _, errOut := run(t, api, false, "projections", "rules", "create", "--scenario", "scn_1", "--category", "income", "--type", "salary", "--config", config, "--json")
		if code != int(cli.ExitUsage) || api.created != nil || !strings.Contains(errOut, "API contract") {
			t.Fatalf("config=%s code=%d request=%+v stderr=%q", config, code, api.created, errOut)
		}
	}
}

func TestScenarioPatchDistinguishesOmittedValueAndNull(t *testing.T) {
	api := &fakeAPI{}
	code, _, stderr := run(t, api, false, "projections", "scenarios", "update", "scn_1", "--portfolio", "prt_1", "--name", "Plan", "--json")
	if code != 0 || api.updatedScenario == nil || api.updatedScenario.BirthYear != nil {
		t.Fatalf("name update code=%d body=%+v stderr=%q", code, api.updatedScenario, stderr)
	}
	code, _, stderr = run(t, api, false, "projections", "scenarios", "update", "scn_1", "--portfolio", "prt_1", "--clear-birth-year", "--json")
	if code != 0 || api.updatedScenario == nil || api.updatedScenario.BirthYear == nil || *api.updatedScenario.BirthYear != nil {
		t.Fatalf("clear update code=%d body=%+v stderr=%q", code, api.updatedScenario, stderr)
	}
	encoded, _ := json.Marshal(api.updatedScenario)
	if string(encoded) != `{"birth_year":null}` {
		t.Fatalf("encoded=%s", encoded)
	}
}

func TestScenarioPatchRejectsValueAndClearTogether(t *testing.T) {
	api := &fakeAPI{}
	code, _, stderr := run(t, api, false, "projections", "scenarios", "update", "scn_1", "--portfolio", "prt_1", "--birth-year", "1990", "--clear-birth-year", "--json")
	if code != int(cli.ExitUsage) || api.updatedScenario != nil || !strings.Contains(stderr, "cannot be used together") {
		t.Fatalf("code=%d body=%+v stderr=%q", code, api.updatedScenario, stderr)
	}
}

func TestGeneratedScenarioPatchSendsAuthAndExactJSON(t *testing.T) {
	var authorization, body string
	httpClient := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		authorization = r.Header.Get("Authorization")
		payload, _ := io.ReadAll(r.Body)
		body = string(payload)
		return &http.Response{StatusCode: http.StatusOK, Status: "200 OK", Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(`{}`)), Request: r}, nil
	})}
	client, err := daikuv1.NewClientWithResponses("https://api.example.test", daikuv1.WithHTTPClient(httpClient), daikuv1.WithRequestEditorFn(func(_ context.Context, request *http.Request) error {
		request.Header.Set("Authorization", "Bearer token")
		return nil
	}))
	if err != nil {
		t.Fatal(err)
	}
	api := generatedAPI{client}
	if _, err = api.ScenarioUpdate(context.Background(), "prt_1", "scn_1", scenarioPatch{Name: ptr("Plan")}); err != nil {
		t.Fatal(err)
	}
	if authorization != "Bearer token" || body != `{"name":"Plan"}` {
		t.Fatalf("authorization=%q body=%s", authorization, body)
	}
	if _, err = api.ScenarioUpdate(context.Background(), "prt_1", "scn_1", scenarioPatch{BirthYear: new(*int)}); err != nil {
		t.Fatal(err)
	}
	if body != `{"birth_year":null}` {
		t.Fatalf("clear body=%s", body)
	}
}

func TestHTTPStatusErrorsAreTyped(t *testing.T) {
	tests := []struct {
		status int
		code   string
		exit   cli.ExitCode
	}{
		{http.StatusBadRequest, "usage_error", cli.ExitUsage},
		{http.StatusUnauthorized, "unauthorized", cli.ExitAuth},
		{http.StatusForbidden, "forbidden", cli.ExitForbidden},
		{http.StatusNotFound, "not_found", cli.ExitNotFound},
		{http.StatusTooManyRequests, "api_error", cli.ExitFailure},
	}
	for _, test := range tests {
		err := status(test.status)
		var typed *cli.Error
		if !errors.As(err, &typed) || typed.Code != test.code || typed.ExitCode != test.exit {
			t.Fatalf("status=%d err=%+v", test.status, err)
		}
	}
}

func TestExchangeRatesNotFoundDoesNotClaimPortfolioOrScenarioIsMissing(t *testing.T) {
	httpClient := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusNotFound,
			Status:     "404 Not Found",
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"error":{"errors":null,"message":"rates unavailable","status_code":404}}`)),
			Request:    r,
		}, nil
	})}
	client, err := daikuv1.NewClientWithResponses("https://api.example.test", daikuv1.WithHTTPClient(httpClient))
	if err != nil {
		t.Fatal(err)
	}

	code, _, errOut := run(t, generatedAPI{client}, false, "exchange-rates", "--json")
	if code != int(cli.ExitNotFound) || !strings.Contains(errOut, `"message":"the requested resource was not found"`) {
		t.Fatalf("code=%d stderr=%q", code, errOut)
	}
	if strings.Contains(errOut, "portfolio") || strings.Contains(errOut, "scenario") {
		t.Fatalf("exchange-rate error names an unrelated resource: %q", errOut)
	}
}

func ptr[T any](value T) *T { return &value }

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) { return f(request) }

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

func TestSpanishHumanOutputLocalizesHeadingsButNotValues(t *testing.T) {
	date := types.Date{Time: time.Date(2026, 8, 29, 0, 0, 0, 0, time.UTC)}
	api := &fakeAPI{series: &daikuv1.NetWorthSeries{Currency: "EUR", Series: []daikuv1.NetWorthPoint{{Date: date, NetWorth: "99.10", Assets: "120.20", Liabilities: "21.10"}}}}
	code, out, _ := run(t, api, true, "reports", "net-worth", "--language", "es")
	if code != 0 || !strings.Contains(out, "PATRIMONIO") || !strings.Contains(out, "ACTIVOS") || !strings.Contains(out, "99.10") || !strings.Contains(out, "EUR") {
		t.Fatalf("code=%d out=%q", code, out)
	}
}

func TestSpanishLocalizesProjectionValidationAndFlags(t *testing.T) {
	api := &fakeAPI{}
	code, _, stderr := run(t, api, false, "projections", "scenarios", "update", "scn_1", "--portfolio", "prt_1", "--birth-year", "1800", "--language", "es")
	if code != int(cli.ExitUsage) || !strings.Contains(stderr, "año de nacimiento no es válido") {
		t.Fatalf("code=%d stderr=%q", code, stderr)
	}
	code, stdout, _ := run(t, api, false, "projections", "scenarios", "update", "--help", "--language", "es")
	if code != 0 || !strings.Contains(stdout, "borra el año de nacimiento") {
		t.Fatalf("code=%d stdout=%q", code, stdout)
	}
}
