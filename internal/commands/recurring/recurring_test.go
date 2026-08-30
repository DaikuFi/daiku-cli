package recurring

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
	created          *daikuv1.RecurringExpenseRequest
	updated          Patch
	confirmed        *daikuv1.RecurringOccurrenceConfirmRequestRequest
	skipped, snoozed string
}

func (f *fakeAPI) List(context.Context, string) ([]daikuv1.RecurringExpense, error) {
	return []daikuv1.RecurringExpense{}, nil
}
func (f *fakeAPI) Create(_ context.Context, _ string, b daikuv1.RecurringExpenseRequest) (*daikuv1.RecurringExpense, error) {
	f.created = &b
	return &daikuv1.RecurringExpense{Description: b.Description, Amount: b.Amount, DayOfMonth: b.DayOfMonth, Frequency: b.Frequency}, nil
}
func (f *fakeAPI) Update(_ context.Context, _, _ string, patch Patch) (*daikuv1.RecurringExpense, error) {
	f.updated = patch
	return &daikuv1.RecurringExpense{}, nil
}
func (f *fakeAPI) Delete(context.Context, string, string) error { return nil }
func (f *fakeAPI) Occurrences(context.Context, string, *daikuv1.DaikuHouseholdsHouseholdPkRecurringOccurrencesGetParams) ([]daikuv1.RecurringOccurrence, error) {
	return []daikuv1.RecurringOccurrence{}, nil
}
func (f *fakeAPI) Confirm(_ context.Context, _ string, _ string, b daikuv1.RecurringOccurrenceConfirmRequestRequest) (*daikuv1.RecurringOccurrence, error) {
	f.confirmed = &b
	return &daikuv1.RecurringOccurrence{}, nil
}
func (f *fakeAPI) Skip(_ context.Context, _ string, id string) (*daikuv1.RecurringOccurrence, error) {
	f.skipped = id
	return &daikuv1.RecurringOccurrence{}, nil
}
func (f *fakeAPI) Snooze(_ context.Context, _ string, id string, _ daikuv1.RecurringOccurrenceSnoozeRequestRequest) (*daikuv1.RecurringOccurrence, error) {
	f.snoozed = id
	return &daikuv1.RecurringOccurrence{}, nil
}

func runSimple(t *testing.T, api API, args ...string) (int, string, string) {
	t.Helper()
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	app := cli.New(cli.WithIO(strings.NewReader(""), out, errOut), cli.WithTerminalDetector(func(_ io.Writer) bool { return false }), cli.WithInteractiveDetector(func(_ io.Reader, _ io.Writer) bool { return false }), cli.WithModule(Module{Factory: func(context.Context) (API, error) { return api, nil }}))
	return app.Run(args), out.String(), errOut.String()
}

func TestRegistersNoGenerateCommand(t *testing.T) {
	api := &fakeAPI{}
	code, _, errOut := runSimple(t, api, "recurring", "generate", "--json")
	if code != 2 || (!strings.Contains(errOut, "unknown command") && !strings.Contains(errOut, "unknown flag")) {
		t.Fatalf("code=%d stderr=%q", code, errOut)
	}
}
func TestCreatesTransferTemplateWithoutLocalOccurrence(t *testing.T) {
	api := &fakeAPI{}
	code, _, errOut := runSimple(t, api, "recurring", "create", "--household", "hh_1", "--description", "Rent", "--amount", "100", "--currency", "USD", "--frequency", "monthly", "--type", "transfer", "--creation-mode", "confirm", "--day", "5", "--account", "acc_a", "--destination-account", "acc_b", "--json")
	if code != 0 {
		t.Fatalf("code=%d stderr=%q", code, errOut)
	}
	if api.created == nil || string(*api.created.TransactionType) != "transfer" || string(*api.created.CreationMode) != "confirm" || *api.created.DestinationAccount != "acc_b" {
		t.Fatalf("body=%+v", api.created)
	}
}

func TestCreateDoesNotRegisterPatchOnlyClearFlags(t *testing.T) {
	api := &fakeAPI{}
	base := []string{"recurring", "create", "--household", "hh_1", "--description", "Rent", "--amount", "100", "--currency", "USD", "--frequency", "monthly", "--type", "expense", "--creation-mode", "auto", "--day", "5", "--json"}
	for _, flag := range []string{"--clear-account", "--clear-destination-account", "--clear-category", "--clear-month"} {
		code, _, errOut := runSimple(t, api, append(base, flag)...)
		if code != int(cli.ExitUsage) || !strings.Contains(errOut, "unknown flag: "+flag) {
			t.Fatalf("flag=%s code=%d stderr=%q", flag, code, errOut)
		}
		if api.created != nil {
			t.Fatalf("%s reached the API", flag)
		}
	}
}
func TestConfirmRequiresExplicitNonInteractiveConsent(t *testing.T) {
	api := &fakeAPI{}
	args := []string{"recurring", "occurrences", "confirm", "occ_1", "--household", "hh_1", "--date", "2026-08-30", "--amount", "42", "--json"}
	code, _, errOut := runSimple(t, api, args...)
	if code != 2 || !strings.Contains(errOut, "confirmation_required") {
		t.Fatalf("code=%d stderr=%q", code, errOut)
	}
	if api.confirmed != nil {
		t.Fatal("confirmed without --yes")
	}
	args = append(args, "--yes")
	code, _, errOut = runSimple(t, api, args...)
	if code != 0 {
		t.Fatalf("code=%d stderr=%q", code, errOut)
	}
	if api.confirmed == nil || api.confirmed.FinalAmount != "42" || api.confirmed.FinalDate.Time.Format("2006-01-02") != "2026-08-30" {
		t.Fatalf("body=%+v", api.confirmed)
	}
}
func TestResolvedStatusesAreInspectable(t *testing.T) {
	api := &fakeAPI{}
	for _, status := range []string{"confirmed", "skipped", "superseded", "all"} {
		code, _, errOut := runSimple(t, api, "recurring", "occurrences", "list", "--household", "hh_1", "--status", status, "--json")
		if code != 0 {
			t.Fatalf("status=%s code=%d stderr=%q", status, code, errOut)
		}
	}
}

func TestExpenseIncomeAndOccurrenceActions(t *testing.T) {
	for _, kind := range []string{"expense", "income"} {
		api := &fakeAPI{}
		code, _, errOut := runSimple(t, api, "recurring", "create", "--household", "hh_1", "--description", kind, "--amount", "10", "--currency", "EUR", "--frequency", "monthly", "--type", kind, "--creation-mode", "auto", "--day", "9", "--json")
		if code != 0 || api.created == nil || string(*api.created.TransactionType) != kind {
			t.Fatalf("kind=%s code=%d stderr=%q body=%+v", kind, code, errOut, api.created)
		}
	}
	api := &fakeAPI{}
	code, _, errOut := runSimple(t, api, "recurring", "occurrences", "skip", "occ_skip", "--household", "hh_1", "--yes", "--json")
	if code != 0 || api.skipped != "occ_skip" {
		t.Fatalf("skip code=%d stderr=%q", code, errOut)
	}
	code, _, errOut = runSimple(t, api, "recurring", "occurrences", "snooze", "occ_snooze", "--household", "hh_1", "--until", "2026-09-01", "--json")
	if code != 0 || api.snoozed != "occ_snooze" {
		t.Fatalf("snooze code=%d stderr=%q", code, errOut)
	}
}

func TestUpdateOmissionAndExplicitClears(t *testing.T) {
	api := &fakeAPI{}
	code, _, errOut := runSimple(t, api, "recurring", "update", "rec_1", "--household", "hh_1", "--amount", "12", "--json")
	if code != 0 {
		t.Fatalf("code=%d stderr=%q", code, errOut)
	}
	if len(api.updated) != 1 || api.updated["amount"] != "12" {
		t.Fatalf("patch=%#v", api.updated)
	}
	for _, key := range []string{"account", "category", "destination_account", "month_of_year"} {
		if _, ok := api.updated[key]; ok {
			t.Fatalf("%s must be omitted: %#v", key, api.updated)
		}
	}
	code, _, errOut = runSimple(t, api, "recurring", "update", "rec_1", "--household", "hh_1", "--clear-account", "--clear-category", "--clear-destination-account", "--clear-month", "--json")
	if code != 0 {
		t.Fatalf("code=%d stderr=%q", code, errOut)
	}
	for _, key := range []string{"account", "category", "destination_account", "month_of_year"} {
		if value, ok := api.updated[key]; !ok || value != nil {
			t.Fatalf("%s must be null: %#v", key, api.updated)
		}
	}
}

func TestUpdateRejectsInvalidExplicitScheduleValues(t *testing.T) {
	for _, tc := range []struct {
		name  string
		flags []string
	}{
		{name: "zero day", flags: []string{"--day", "0"}},
		{name: "day above range", flags: []string{"--day", "32"}},
		{name: "zero month", flags: []string{"--month", "0"}},
		{name: "month above range", flags: []string{"--month", "13"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			api := &fakeAPI{}
			args := []string{"recurring", "update", "rec_1", "--household", "hh_1", "--json"}
			args = append(args, tc.flags...)
			code, _, errOut := runSimple(t, api, args...)
			if code != int(cli.ExitUsage) || errOut == "" {
				t.Fatalf("code=%d stderr=%q", code, errOut)
			}
			if api.updated != nil {
				t.Fatalf("API reached with invalid schedule: %#v", api.updated)
			}
		})
	}
}

func TestAcceptsPublishedCurrencyAndRejectsUnknown(t *testing.T) {
	api := &fakeAPI{}
	base := []string{"recurring", "create", "--household", "hh_1", "--description", "x", "--amount", "10", "--frequency", "monthly", "--type", "expense", "--creation-mode", "auto", "--day", "1", "--json", "--currency"}
	code, _, errOut := runSimple(t, api, append(base, "MXN")...)
	if code != 0 {
		t.Fatalf("MXN code=%d stderr=%q", code, errOut)
	}
	code, _, errOut = runSimple(t, api, append(base, "XYZ")...)
	if code != 2 || !strings.Contains(errOut, "not supported") {
		t.Fatalf("XYZ code=%d stderr=%q", code, errOut)
	}
}

func TestStatusMapsRolesAndResolvedConflict(t *testing.T) {
	for code, want := range map[int]cli.ExitCode{403: cli.ExitForbidden, 409: cli.ExitConflict} {
		err, ok := status(code).(*cli.Error)
		if !ok || err.ExitCode != want {
			t.Fatalf("status(%d)=%#v", code, err)
		}
	}
}

func TestSpanishValidationAndFlagHelp(t *testing.T) {
	api := &fakeAPI{}
	code, out, errOut := runSimple(t, api, "recurring", "create", "--household", "hh_1", "--description", "x", "--amount", "10", "--currency", "UYU", "--frequency", "weekly", "--type", "expense", "--creation-mode", "auto", "--day", "1", "--language", "es")
	if code != int(cli.ExitUsage) || out != "" || !strings.Contains(errOut, "la frecuencia debe ser monthly o yearly") {
		t.Fatalf("code=%d out=%q stderr=%q", code, out, errOut)
	}
	code, out, errOut = runSimple(t, api, "recurring", "update", "--help", "--language", "es")
	if code != 0 || errOut != "" || !strings.Contains(out, "elimina la cuenta de origen") || !strings.Contains(out, "descripción") {
		t.Fatalf("code=%d out=%q stderr=%q", code, out, errOut)
	}
}
