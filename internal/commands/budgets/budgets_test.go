package budgets

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"

	daikuv1 "github.com/DaikuFi/daiku-cli/generated/daikuv1"
	"github.com/DaikuFi/daiku-cli/internal/cli"
)

type fakeAPI struct {
	summary     *daikuv1.DaikuHouseholdsHouseholdPkBudgetsSummaryGetParams
	planned     *daikuv1.DaikuHouseholdsHouseholdPkBudgetsPlannedGetParams
	suggestions *daikuv1.DaikuHouseholdsHouseholdPkBudgetsSuggestionsGetParams
	created     *daikuv1.CategoryBudgetRequest
	deleted     string
	updated     Patch
}

func (f *fakeAPI) Planned(_ context.Context, _ string, params *daikuv1.DaikuHouseholdsHouseholdPkBudgetsPlannedGetParams) (*daikuv1.PlannedBudgets, error) {
	f.planned = params
	return &daikuv1.PlannedBudgets{}, nil
}
func (f *fakeAPI) Suggestions(_ context.Context, _ string, params *daikuv1.DaikuHouseholdsHouseholdPkBudgetsSuggestionsGetParams) (*daikuv1.BudgetSuggestionsResponse, error) {
	f.suggestions = params
	return &daikuv1.BudgetSuggestionsResponse{}, nil
}
func (f *fakeAPI) Summary(_ context.Context, _ string, p *daikuv1.DaikuHouseholdsHouseholdPkBudgetsSummaryGetParams) (*daikuv1.BudgetSummary, error) {
	f.summary = p
	return &daikuv1.BudgetSummary{}, nil
}
func (f *fakeAPI) List(context.Context, string) ([]daikuv1.CategoryBudget, error) {
	return []daikuv1.CategoryBudget{}, nil
}
func (f *fakeAPI) Create(_ context.Context, _ string, b daikuv1.CategoryBudgetRequest) (*daikuv1.CategoryBudget, error) {
	f.created = &b
	return &daikuv1.CategoryBudget{Amount: b.Amount, Category: b.Category}, nil
}
func (f *fakeAPI) Update(_ context.Context, _, _ string, patch Patch) (*daikuv1.CategoryBudget, error) {
	f.updated = patch
	return &daikuv1.CategoryBudget{}, nil
}

func TestUpdatePreservesOmittedAndExplicitNull(t *testing.T) {
	api := &fakeAPI{}
	code, _, errOut := runSimple(t, api, "budgets", "rules", "update", "bud_1", "--household", "hh_1", "--amount", "123", "--json")
	if code != 0 {
		t.Fatalf("code=%d stderr=%q", code, errOut)
	}
	if len(api.updated) != 1 || api.updated["amount"] != "123" {
		t.Fatalf("patch=%#v", api.updated)
	}
	if _, ok := api.updated["month"]; ok {
		t.Fatalf("month must be omitted: %#v", api.updated)
	}
	code, _, errOut = runSimple(t, api, "budgets", "rules", "update", "bud_1", "--household", "hh_1", "--clear-month", "--clear-year", "--json")
	if code != 0 {
		t.Fatalf("code=%d stderr=%q", code, errOut)
	}
	if month, ok := api.updated["month"]; !ok || month != nil {
		t.Fatalf("month clear=%#v", api.updated)
	}
	if year, ok := api.updated["year"]; !ok || year != nil {
		t.Fatalf("year clear=%#v", api.updated)
	}
}

func TestBudgetRuleAcceptsPublishedCurrency(t *testing.T) {
	api := &fakeAPI{}
	code, _, errOut := runSimple(t, api, "budgets", "rules", "create", "--household", "hh_1", "--category", "cat_1", "--amount", "10", "--currency", "BRL", "--scope", "monthly", "--json")
	if code != 0 {
		t.Fatalf("code=%d stderr=%q", code, errOut)
	}
}

func TestPlannedAndSuggestionsPinTheirPeriods(t *testing.T) {
	api := &fakeAPI{}
	code, _, errOut := runSimple(t, api, "budgets", "planned", "--household", "hh_1", "--year", "2027", "--currency", "EUR", "--json")
	if code != 0 || api.planned == nil || *api.planned.Year != 2027 {
		t.Fatalf("planned code=%d stderr=%q params=%+v", code, errOut, api.planned)
	}
	code, _, errOut = runSimple(t, api, "budgets", "suggestions", "--household", "hh_1", "--year", "2027", "--month", "4", "--currency", "USD", "--json")
	if code != 0 || api.suggestions == nil || *api.suggestions.Year != 2027 || *api.suggestions.Month != 4 {
		t.Fatalf("suggestions code=%d stderr=%q params=%+v", code, errOut, api.suggestions)
	}
}

func TestSpanishHumanOutputAndForbiddenError(t *testing.T) {
	api := &fakeAPI{}
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	app := cli.New(cli.WithIO(strings.NewReader(""), out, errOut), cli.WithEnvironment(func(name string) (string, bool) {
		if name == "DAIKU_LANG" {
			return "es", true
		}
		return "", false
	}), cli.WithTerminalDetector(func(io.Writer) bool { return false }), cli.WithInteractiveDetector(func(io.Reader, io.Writer) bool { return false }), cli.WithModule(Module{Factory: func(context.Context) (API, error) { return api, nil }}))
	if code := app.Run([]string{"budgets", "rules", "list", "--household", "hh_1"}); code != 0 || !strings.Contains(out.String(), "reglas de presupuesto") {
		t.Fatalf("code=%d out=%q err=%q", code, out.String(), errOut.String())
	}
	err := status(403)
	cliErr, ok := err.(*cli.Error)
	if !ok || cliErr.ExitCode != cli.ExitForbidden || cliErr.Code != "forbidden" {
		t.Fatalf("error=%#v", err)
	}
}
func (f *fakeAPI) Delete(_ context.Context, _ string, id string) error { f.deleted = id; return nil }

func TestSummaryPinsPeriodAndCurrency(t *testing.T) {
	api := &fakeAPI{}
	code, out, _ := runSimple(t, api, "budgets", "summary", "--household", "hh_1", "--month", "8", "--year", "2026", "--currency", "UYU", "--json")
	if code != 0 {
		t.Fatalf("code=%d", code)
	}
	if api.summary == nil || *api.summary.Month != 8 || *api.summary.Year != 2026 || string(*api.summary.DisplayCurrency) != "UYU" {
		t.Fatalf("params=%+v", api.summary)
	}
	if out != "{\"ok\":true,\"data\":{\"budgeted_spent\":\"\",\"category_rows\":null,\"daily_progress\":null,\"days_elapsed\":0,\"days_in_month\":0,\"display_currency\":\"\",\"free_to_spend\":null,\"pace_status\":null,\"total_monthly_budget\":\"\",\"total_spent\":\"\",\"unconverted\":null}}\n" {
		t.Fatalf("unexpected JSON: %q", out)
	}
}
func TestMonthScopeRequiresPinnedYear(t *testing.T) {
	api := &fakeAPI{}
	code, _, errOut := runSimple(t, api, "budgets", "rules", "create", "--household", "hh_1", "--category", "cat_1", "--amount", "100", "--currency", "USD", "--scope", "month", "--month", "8", "--json")
	if code != 2 || !strings.Contains(errOut, "month scope requires") {
		t.Fatalf("code=%d stderr=%q", code, errOut)
	}
	if api.created != nil {
		t.Fatal("API called after invalid period")
	}
}
func TestDeleteIsSafeWithoutTTY(t *testing.T) {
	api := &fakeAPI{}
	code, _, errOut := runSimple(t, api, "budgets", "rules", "delete", "bud_1", "--household", "hh_1", "--json")
	if code != 2 || !strings.Contains(errOut, "confirmation_required") {
		t.Fatalf("code=%d stderr=%q", code, errOut)
	}
	if api.deleted != "" {
		t.Fatal("delete called without confirmation")
	}
}

func runSimple(t *testing.T, api API, args ...string) (int, string, string) {
	t.Helper()
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	app := cli.New(cli.WithIO(strings.NewReader(""), out, errOut), cli.WithTerminalDetector(func(_ io.Writer) bool { return false }), cli.WithInteractiveDetector(func(_ io.Reader, _ io.Writer) bool { return false }), cli.WithModule(Module{Factory: func(context.Context) (API, error) { return api, nil }}))
	return app.Run(args), out.String(), errOut.String()
}
