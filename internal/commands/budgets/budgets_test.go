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
	summary *daikuv1.DaikuHouseholdsHouseholdPkBudgetsSummaryGetParams
	created *daikuv1.CategoryBudgetRequest
	deleted string
}

func (f *fakeAPI) Planned(context.Context, string, *daikuv1.DaikuHouseholdsHouseholdPkBudgetsPlannedGetParams) (*daikuv1.PlannedBudgets, error) {
	return &daikuv1.PlannedBudgets{}, nil
}
func (f *fakeAPI) Suggestions(context.Context, string, *daikuv1.DaikuHouseholdsHouseholdPkBudgetsSuggestionsGetParams) (*daikuv1.BudgetSuggestionsResponse, error) {
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
func (f *fakeAPI) Update(context.Context, string, string, daikuv1.PatchedCategoryBudgetRequest) (*daikuv1.CategoryBudget, error) {
	return &daikuv1.CategoryBudget{}, nil
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
