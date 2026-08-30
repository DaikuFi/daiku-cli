package budgets

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	daikuv1 "github.com/DaikuFi/daiku-cli/generated/daikuv1"
	"github.com/DaikuFi/daiku-cli/internal/cli"
)

func testGeneratedAPI(t *testing.T, handler http.HandlerFunc) generatedAPI {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	client, err := daikuv1.NewClientWithResponses(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	return generatedAPI{client}
}

func TestGeneratedAPIUsesExactBudgetURLsAndPatchNullSemantics(t *testing.T) {
	var requests []string
	var patch map[string]any
	api := testGeneratedAPI(t, func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, r.Method+" "+r.URL.RequestURI())
		w.Header().Set("Content-Type", "application/json")
		switch r.Method {
		case http.MethodGet:
			_, _ = io.WriteString(w, `[]`)
		case http.MethodPatch:
			if err := json.NewDecoder(r.Body).Decode(&patch); err != nil {
				t.Fatal(err)
			}
			_, _ = io.WriteString(w, `{}`)
		case http.MethodDelete:
			w.WriteHeader(http.StatusNoContent)
		}
	})
	if _, err := api.List(context.Background(), "hh_1"); err != nil {
		t.Fatal(err)
	}
	if _, err := api.Update(context.Background(), "hh_1", "bud_1", Patch{"amount": "12.00", "month": nil}); err != nil {
		t.Fatal(err)
	}
	if err := api.Delete(context.Background(), "hh_1", "bud_1"); err != nil {
		t.Fatal(err)
	}
	want := []string{"GET /api/v1/households/hh_1/category-budgets/", "PATCH /api/v1/households/hh_1/category-budgets/bud_1/", "DELETE /api/v1/households/hh_1/category-budgets/bud_1/"}
	if strings.Join(requests, "\n") != strings.Join(want, "\n") {
		t.Fatalf("requests=%q", requests)
	}
	if len(patch) != 2 || patch["amount"] != "12.00" {
		t.Fatalf("patch=%#v", patch)
	}
	if month, ok := patch["month"]; !ok || month != nil {
		t.Fatalf("month must be explicit null: %#v", patch)
	}
	if _, ok := patch["year"]; ok {
		t.Fatalf("year must be omitted: %#v", patch)
	}
}

func TestGeneratedBudgetAPIMapsHTTPStatuses(t *testing.T) {
	for _, tc := range []struct {
		status int
		exit   cli.ExitCode
		code   string
	}{{401, cli.ExitAuth, "unauthorized"}, {403, cli.ExitForbidden, "forbidden"}, {404, cli.ExitNotFound, "not_found"}} {
		t.Run(tc.code, func(t *testing.T) {
			api := testGeneratedAPI(t, func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(tc.status) })
			_, err := api.List(context.Background(), "hh_1")
			cliErr, ok := err.(*cli.Error)
			if !ok || cliErr.ExitCode != tc.exit || cliErr.Code != tc.code {
				t.Fatalf("error=%#v", err)
			}
		})
	}
}

func TestGeneratedBudgetAPIDecodesBackendSummaryWithUnconvertedCurrencies(t *testing.T) {
	api := testGeneratedAPI(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{
			"budgeted_spent":"80.00",
			"category_rows":[],
			"daily_progress":[],
			"days_elapsed":30,
			"days_in_month":31,
			"display_currency":"UYU",
			"free_to_spend":"20.00",
			"pace_status":"ok",
			"total_monthly_budget":"100.00",
			"total_spent":"80.00",
			"unconverted":{"count":1,"currencies":["USD"]}
		}`)
	})

	summary, err := api.Summary(context.Background(), "hh_1", nil)
	if err != nil {
		t.Fatalf("Summary returned error for the backend wire response: %v", err)
	}
	if summary.TotalMonthlyBudget != "100.00" {
		t.Fatalf("summary=%#v", summary)
	}
	if summary.Unconverted == nil || summary.Unconverted.Count != 1 || strings.Join(summary.Unconverted.Currencies, ",") != "USD" {
		t.Fatalf("unconverted=%#v", summary.Unconverted)
	}
}

func TestGeneratedBudgetAPIDecodesNullUnconvertedSummary(t *testing.T) {
	api := testGeneratedAPI(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{
			"budgeted_spent":"80.00","category_rows":[],"daily_progress":[],
			"days_elapsed":30,"days_in_month":31,"display_currency":"UYU",
			"free_to_spend":"20.00","pace_status":"ok",
			"total_monthly_budget":"100.00","total_spent":"80.00","unconverted":null
		}`)
	})

	summary, err := api.Summary(context.Background(), "hh_1", nil)
	if err != nil {
		t.Fatal(err)
	}
	if summary.Unconverted != nil {
		t.Fatalf("unconverted=%#v", summary.Unconverted)
	}
}

func TestGeneratedBudgetAPIReturnsStructuredSummaryErrors(t *testing.T) {
	t.Run("server error", func(t *testing.T) {
		api := testGeneratedAPI(t, func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_, _ = io.WriteString(w, `{"error":{"errors":{"month":["Must be valid."]},"message":"Invalid period","status_code":400}}`)
		})

		_, err := api.Summary(context.Background(), "hh_1", nil)
		cliErr, ok := err.(*cli.Error)
		if !ok || cliErr.Code != "invalid_request" || cliErr.ExitCode != cli.ExitUsage {
			t.Fatalf("error=%#v", err)
		}
		if cliErr.Message != "Daiku API returned HTTP 400: Invalid period" {
			t.Fatalf("message=%q", cliErr.Message)
		}
		details, ok := cliErr.Details.(map[string]any)
		if !ok || details["operation"] != "budget_summary" || details["status_code"] != http.StatusBadRequest || details["errors"] == nil {
			t.Fatalf("details=%#v", cliErr.Details)
		}
	})

	t.Run("invalid success response", func(t *testing.T) {
		api := testGeneratedAPI(t, func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"unconverted":[]}`)
		})

		_, err := api.Summary(context.Background(), "hh_1", nil)
		cliErr, ok := err.(*cli.Error)
		if !ok || cliErr.Code != "invalid_response" || cliErr.ExitCode != cli.ExitFailure {
			t.Fatalf("error=%#v", err)
		}
		details, ok := cliErr.Details.(map[string]any)
		if !ok || details["operation"] != "budget_summary" || details["reason"] == nil {
			t.Fatalf("details=%#v", cliErr.Details)
		}
	})
}
