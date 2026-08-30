package transactions

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"strings"
	"testing"

	"github.com/DaikuFi/daiku-cli/generated/daikuv1"
	"github.com/DaikuFi/daiku-cli/internal/cli"
)

type fakeService struct {
	created            *daikuv1.ExpenseRequest
	transfer           *daikuv1.TransferCreateRequestRequest
	installment        *daikuv1.InstallmentCreateRequestRequest
	bulk               *daikuv1.ExpenseBulkCreateRequestRequest
	updated            PatchBody
	installmentUpdated PatchBody
	bulkUpdated        *BulkUpdateBody
	deleted            bool
	bulkDeleted        bool
	converted          bool
	unlinked           bool
	listed             *daikuv1.DaikuHouseholdsHouseholdPkExpensesGetParams
	listResult         any
	transactionHH      string
	transactionID      string
	installmentHH      string
	installmentList    any
}

func (f *fakeService) List(_ context.Context, _ string, p *daikuv1.DaikuHouseholdsHouseholdPkExpensesGetParams) (any, error) {
	f.listed = p
	if f.listResult != nil {
		return f.listResult, nil
	}
	return []daikuv1.Expense{}, nil
}
func (f *fakeService) GetTransaction(_ context.Context, hh, id string) (any, error) {
	f.transactionHH, f.transactionID = hh, id
	return daikuv1.Expense{Id: &id, Amount: "10.00", Description: "Coffee"}, nil
}
func (f *fakeService) Create(_ context.Context, _ string, b daikuv1.ExpenseRequest) (any, error) {
	f.created = &b
	return daikuv1.Expense{Amount: b.Amount, Description: b.Description}, nil
}
func (f *fakeService) Update(_ context.Context, _, _ string, body PatchBody) (any, error) {
	f.updated = body
	return daikuv1.Expense{}, nil
}
func (f *fakeService) Delete(context.Context, string, string, *daikuv1.DaikuHouseholdsHouseholdPkExpensesIdDeleteParams) error {
	f.deleted = true
	return nil
}
func (f *fakeService) BulkCreate(_ context.Context, _ string, body daikuv1.ExpenseBulkCreateRequestRequest) (any, error) {
	f.bulk = &body
	return []daikuv1.Expense{}, nil
}
func (f *fakeService) BulkUpdate(_ context.Context, _ string, body BulkUpdateBody) (any, error) {
	f.bulkUpdated = &body
	return daikuv1.ExpenseBulkUpdateResponse{}, nil
}
func (f *fakeService) BulkDelete(context.Context, string) (any, error) {
	f.bulkDeleted = true
	return daikuv1.DeletedCount{}, nil
}
func (f *fakeService) CreateTransfer(_ context.Context, _ string, b daikuv1.TransferCreateRequestRequest) (any, error) {
	f.transfer = &b
	return daikuv1.TransferResponse{}, nil
}
func (f *fakeService) ConvertTransfer(context.Context, string, string, daikuv1.TransferConvertRequestRequest) (any, error) {
	f.converted = true
	return daikuv1.TransferResponse{}, nil
}
func (*fakeService) TransferCandidates(context.Context, string, string) (any, error) {
	return []daikuv1.Expense{}, nil
}
func (f *fakeService) UnlinkTransfer(context.Context, string, string) (any, error) {
	f.unlinked = true
	return daikuv1.TransferUnlinkResponse{}, nil
}
func (f *fakeService) CreateInstallments(_ context.Context, _ string, body daikuv1.InstallmentCreateRequestRequest) (any, error) {
	f.installment = &body
	return InstallmentPlanResponse{}, nil
}
func (f *fakeService) ListInstallments(_ context.Context, hh string) (any, error) {
	f.installmentHH = hh
	if f.installmentList != nil {
		return f.installmentList, nil
	}
	return []InstallmentPlanResponse{}, nil
}
func (*fakeService) GetInstallment(context.Context, string, string) (any, error) {
	return InstallmentPlanResponse{}, nil
}
func (f *fakeService) UpdateInstallment(_ context.Context, _, _ string, body PatchBody) (any, error) {
	f.installmentUpdated = body
	return InstallmentPlanResponse{}, nil
}

func run(t *testing.T, svc *fakeService, input string, args ...string) (int, string, string) {
	t.Helper()
	var out, errOut bytes.Buffer
	app := cli.New(cli.WithIO(bytes.NewBufferString(input), &out, &errOut), cli.WithInteractiveDetector(func(_ io.Reader, _ io.Writer) bool { return false }), cli.WithModule(New(func(context.Context) (Service, error) { return svc, nil })))
	code := app.Run(args)
	return code, out.String(), errOut.String()
}

func stringptr(value string) *string { return &value }

func TestCreatePreservesDecimalStringAndJSONEnvelope(t *testing.T) {
	svc := &fakeService{}
	code, out, stderr := run(t, svc, "", "transactions", "create", "--household", "hh_1", "--amount", "1.2300", "--description", "Lunch", "--currency", "USD", "--json")
	if code != int(cli.ExitUsage) {
		t.Fatalf("code=%d out=%s err=%s", code, out, stderr)
	}
	code, out, stderr = run(t, svc, "", "transactions", "create", "--household", "hh_1", "--amount", "1.20", "--description", "Lunch", "--currency", "USD", "--json")
	if code != 0 {
		t.Fatalf("code=%d err=%s", code, stderr)
	}
	if svc.created == nil || svc.created.Amount != "1.20" {
		t.Fatalf("amount changed: %#v", svc.created)
	}
	var envelope map[string]any
	if json.Unmarshal([]byte(out), &envelope) != nil || envelope["ok"] != true {
		t.Fatalf("invalid envelope %q", out)
	}
}

func TestListRejectsInvertedRangeBeforeCallingAPI(t *testing.T) {
	svc := &fakeService{}
	code, _, stderr := run(t, svc, "", "transactions", "list", "--household", "hh_1", "--from", "2026-08-30", "--to", "2026-08-01")
	if code != int(cli.ExitUsage) || svc.listed != nil {
		t.Fatalf("code=%d listed=%v err=%s", code, svc.listed, stderr)
	}
}

func TestListWiresPublishedMonthYearAndAllControls(t *testing.T) {
	svc := &fakeService{}
	code, _, stderr := run(t, svc, "", "transactions", "list", "--household", "hh_1", "--month", "8", "--year", "2026", "--all", "--json")
	if code != 0 {
		t.Fatalf("code=%d err=%s", code, stderr)
	}
	if svc.listed == nil || svc.listed.Month == nil || *svc.listed.Month != 8 || svc.listed.Year == nil || *svc.listed.Year != 2026 {
		t.Fatalf("params=%#v", svc.listed)
	}
	if svc.listed.Paginated != nil {
		t.Fatalf("--all sent paginated=%v; the contract requires omitting it", *svc.listed.Paginated)
	}

	svc = &fakeService{}
	code, _, stderr = run(t, svc, "", "transactions", "list", "--household", "hh_1", "--json")
	if code != 0 || svc.listed == nil || svc.listed.Paginated == nil || !*svc.listed.Paginated {
		t.Fatalf("default code=%d params=%#v err=%s", code, svc.listed, stderr)
	}

	svc = &fakeService{}
	code, _, stderr = run(t, svc, "", "transactions", "list", "--household", "hh_1", "--currency", "BRL", "--page", "2", "--page-size", "25", "--json")
	if code != 0 || svc.listed == nil {
		t.Fatalf("filtered code=%d params=%#v err=%s", code, svc.listed, stderr)
	}
	if svc.listed.Currency == nil || *svc.listed.Currency != daikuv1.DaikuHouseholdsHouseholdPkExpensesGetParamsCurrency("BRL") || svc.listed.Page == nil || *svc.listed.Page != 2 || svc.listed.PageSize == nil || *svc.listed.PageSize != 25 {
		t.Fatalf("filtered params=%#v", svc.listed)
	}
}

func TestListAcceptsTransferFilterInJSONAndHumanOutput(t *testing.T) {
	id := "exp_transfer"
	typeTransfer := daikuv1.ExpenseTransactionTypeEnumTransfer
	result := daikuv1.ExpensePage{
		Count: 1,
		Results: []daikuv1.Expense{{
			Id: &id, Amount: "40.00", Description: "Checking to Savings",
			TransactionType: &typeTransfer,
		}},
	}
	svc := &fakeService{listResult: result}
	code, out, stderr := run(t, svc, "", "transactions", "list", "--household", "hh_1", "--type", "transfer", "--json")
	if code != 0 || svc.listed == nil || svc.listed.Type == nil || *svc.listed.Type != daikuv1.DaikuHouseholdsHouseholdPkExpensesGetParamsTypeTransfer {
		t.Fatalf("code=%d params=%#v out=%s err=%s", code, svc.listed, out, stderr)
	}
	var envelope struct {
		OK   bool                `json:"ok"`
		Data daikuv1.ExpensePage `json:"data"`
	}
	if err := json.Unmarshal([]byte(out), &envelope); err != nil || !envelope.OK || len(envelope.Data.Results) != 1 {
		t.Fatalf("envelope=%s err=%v", out, err)
	}

	svc = &fakeService{listResult: result}
	code, out, stderr = run(t, svc, "", "transactions", "list", "--household", "hh_1", "--type", "transfer", "--language", "es")
	if code != 0 || !strings.Contains(out, "Checking to Savings") || !strings.Contains(out, "transfer") || !strings.Contains(out, "Tipo") {
		t.Fatalf("code=%d out=%s err=%s", code, out, stderr)
	}
}

func TestListRejectsUnsupportedTypeBeforeCallingAPIInJSONAndHumanOutput(t *testing.T) {
	svc := &fakeService{}
	code, out, stderr := run(t, svc, "", "transactions", "list", "--household", "hh_1", "--type", "adjustment", "--json")
	if code != int(cli.ExitUsage) || svc.listed != nil || out != "" {
		t.Fatalf("code=%d params=%#v out=%s err=%s", code, svc.listed, out, stderr)
	}
	var envelope map[string]any
	if err := json.Unmarshal([]byte(stderr), &envelope); err != nil || envelope["ok"] != false {
		t.Fatalf("json=%s err=%v", stderr, err)
	}
	errorBody, _ := envelope["error"].(map[string]any)
	if errorBody["code"] != "usage_error" || errorBody["message"] != "type must be expense, income, or transfer" {
		t.Fatalf("error=%#v", errorBody)
	}

	svc = &fakeService{}
	code, out, stderr = run(t, svc, "", "transactions", "list", "--household", "hh_1", "--type", "unknown", "--language", "es")
	if code != int(cli.ExitUsage) || svc.listed != nil || out != "" || !strings.Contains(stderr, "type debe ser expense, income o transfer") {
		t.Fatalf("code=%d params=%#v out=%s err=%s", code, svc.listed, out, stderr)
	}
}

func TestListRejectsInvalidOrContradictoryPageControls(t *testing.T) {
	for _, args := range [][]string{
		{"transactions", "list", "--household", "hh_1", "--all", "--page", "2", "--json"},
		{"transactions", "list", "--household", "hh_1", "--all", "--page-size", "25", "--json"},
		{"transactions", "list", "--household", "hh_1", "--page", "0", "--json"},
		{"transactions", "list", "--household", "hh_1", "--page-size", "201", "--json"},
		{"transactions", "list", "--household", "hh_1", "--currency", "ZZZ", "--json"},
	} {
		svc := &fakeService{}
		code, _, _ := run(t, svc, "", args...)
		if code != int(cli.ExitUsage) || svc.listed != nil {
			t.Fatalf("args=%v code=%d params=%#v", args, code, svc.listed)
		}
	}
}

func TestTransactionGetWiresHouseholdAndStableJSON(t *testing.T) {
	svc := &fakeService{}
	code, out, stderr := run(t, svc, "", "transactions", "get", "exp_1", "--household", "hh_1", "--json")
	if code != 0 || svc.transactionHH != "hh_1" || svc.transactionID != "exp_1" {
		t.Fatalf("code=%d household=%q id=%q err=%s", code, svc.transactionHH, svc.transactionID, stderr)
	}
	if !strings.Contains(out, `"ok":true`) || !strings.Contains(out, `"id":"exp_1"`) || !strings.Contains(out, `"amount":"10.00"`) {
		t.Fatalf("json=%s", out)
	}
}

func TestInstallmentListPreservesScheduleJSONAndHumanSummary(t *testing.T) {
	id := "exp_1"
	plan := InstallmentPlanResponse{
		ID: "inp_1", Household: "hh_1", Description: "Laptop", Amount: "1200.00",
		Currency: daikuv1.Currency3e8EnumBRL, Count: 12, ChargedCount: 1, StartDate: "2026-08-30", IsActive: true,
		Schedule: []InstallmentScheduleResponse{{Number: 1, Amount: "100.00", Date: "2026-08-30", Expense: &daikuv1.Expense{Id: &id, Amount: "100.00", Description: "Laptop"}}},
		Tags:     []daikuv1.Tag{},
	}
	svc := &fakeService{installmentList: []InstallmentPlanResponse{plan}}
	code, out, stderr := run(t, svc, "", "installments", "list", "--household", "hh_1", "--json")
	if code != 0 || svc.installmentHH != "hh_1" {
		t.Fatalf("code=%d household=%q err=%s", code, svc.installmentHH, stderr)
	}
	if !strings.Contains(out, `"schedule":[{"amount":"100.00","date":"2026-08-30","expense":`) || !strings.Contains(out, `"number":1`) {
		t.Fatalf("json=%s", out)
	}

	svc = &fakeService{installmentList: []InstallmentPlanResponse{plan}}
	code, out, stderr = run(t, svc, "", "installments", "list", "--household", "hh_1", "--language", "es")
	if code != 0 || !strings.Contains(out, "Laptop") || !strings.Contains(out, "1/12") || !strings.Contains(out, "BRL") {
		t.Fatalf("code=%d out=%s err=%s", code, out, stderr)
	}
}

func TestListRejectsInvalidPublishedCalendarFilters(t *testing.T) {
	for _, args := range [][]string{
		{"transactions", "list", "--household", "hh_1", "--month", "0", "--json"},
		{"transactions", "list", "--household", "hh_1", "--month", "13", "--json"},
		{"transactions", "list", "--household", "hh_1", "--year", "0", "--json"},
	} {
		svc := &fakeService{}
		code, _, _ := run(t, svc, "", args...)
		if code != int(cli.ExitUsage) || svc.listed != nil {
			t.Fatalf("args=%v code=%d params=%#v", args, code, svc.listed)
		}
	}
}

func TestListPreservesPageEnvelopeForAgentsAndGuidesHumansToAll(t *testing.T) {
	id := "exp_1"
	page := daikuv1.ExpensePage{
		Count: 2,
		Next:  stringptr("https://api.daiku.test/api/v1/households/hh_1/expenses/?page=2"),
		Results: []daikuv1.Expense{{
			Id: &id, Amount: "1.00", Description: "Coffee",
		}},
	}
	svc := &fakeService{listResult: page}
	code, out, stderr := run(t, svc, "", "transactions", "list", "--household", "hh_1", "--json")
	if code != 0 {
		t.Fatalf("code=%d err=%s", code, stderr)
	}
	var envelope struct {
		OK   bool                `json:"ok"`
		Data daikuv1.ExpensePage `json:"data"`
	}
	if err := json.Unmarshal([]byte(out), &envelope); err != nil || !envelope.OK || envelope.Data.Count != 2 || envelope.Data.Next == nil {
		t.Fatalf("envelope=%s err=%v", out, err)
	}

	svc = &fakeService{listResult: page}
	code, out, stderr = run(t, svc, "", "transactions", "list", "--household", "hh_1", "--language", "es")
	if code != 0 || !strings.Contains(out, "Mostrando 1 de 2 transacciones") || !strings.Contains(out, "--all") {
		t.Fatalf("code=%d out=%s err=%s", code, out, stderr)
	}
}

func TestDeleteNeedsYesWhenNonInteractive(t *testing.T) {
	svc := &fakeService{}
	code, _, _ := run(t, svc, "", "transactions", "delete", "exp_1", "--household", "hh_1", "--json")
	if code != int(cli.ExitUsage) || svc.deleted {
		t.Fatalf("code=%d deleted=%v", code, svc.deleted)
	}
	code, _, _ = run(t, svc, "", "transactions", "delete", "exp_1", "--household", "hh_1", "--scope", "future", "--yes", "--json")
	if code != 0 || !svc.deleted {
		t.Fatalf("code=%d deleted=%v", code, svc.deleted)
	}
}

func TestTransferKeepsExplicitLegAmounts(t *testing.T) {
	svc := &fakeService{}
	code, _, stderr := run(t, svc, "", "transfers", "create", "--household", "hh_1", "--from-account", "acc_a", "--to-account", "acc_b", "--amount", "100.00", "--to-amount", "2.50", "--json")
	if code != 0 {
		t.Fatalf("code=%d err=%s", code, stderr)
	}
	if svc.transfer == nil || svc.transfer.Amount != "100.00" || svc.transfer.ToAmount == nil || *svc.transfer.ToAmount != "2.50" {
		t.Fatalf("body=%#v", svc.transfer)
	}
}

func TestTransferRejectsSameAccount(t *testing.T) {
	svc := &fakeService{}
	code, _, _ := run(t, svc, "", "transfers", "create", "--household", "hh_1", "--from-account", "acc_a", "--to-account", "acc_a", "--amount", "1", "--json")
	if code != int(cli.ExitUsage) || svc.transfer != nil {
		t.Fatalf("code=%d body=%#v", code, svc.transfer)
	}
}

func TestInstallmentsSendPurchaseTotalAndCount(t *testing.T) {
	svc := &fakeService{}
	code, _, stderr := run(t, svc, "", "installments", "create", "--household", "hh_1", "--amount", "1200.00", "--description", "Laptop", "--currency", "BRL", "--date", "2026-08-29", "--count", "12", "--tag-ids", "tag_1,tag_2", "--json")
	if code != 0 {
		t.Fatalf("code=%d err=%s", code, stderr)
	}
	if svc.installment == nil || svc.installment.Amount != "1200.00" || svc.installment.Installments != 12 || svc.installment.Currency != daikuv1.Currency3e8EnumBRL {
		t.Fatalf("body=%#v", svc.installment)
	}
	if svc.installment.TagIds == nil || len(*svc.installment.TagIds) != 2 {
		t.Fatalf("tags=%#v", svc.installment.TagIds)
	}
}

func TestInstallmentsRequireAtLeastOneCentPerInstallment(t *testing.T) {
	svc := &fakeService{}
	code, _, stderr := run(t, svc, "", "installments", "create", "--household", "hh_1", "--amount", "0.01", "--description", "Purchase", "--currency", "UYU", "--date", "2026-08-29", "--count", "2", "--json")
	if code != int(cli.ExitUsage) || svc.installment != nil {
		t.Fatalf("code=%d body=%#v err=%s", code, svc.installment, stderr)
	}
	code, _, stderr = run(t, svc, "", "installments", "create", "--household", "hh_1", "--amount", "0.60", "--description", "Purchase", "--currency", "UYU", "--date", "2026-08-29", "--count", "60", "--json")
	if code != 0 || svc.installment == nil {
		t.Fatalf("boundary code=%d body=%#v err=%s", code, svc.installment, stderr)
	}
}

func TestBulkCreateUsesContractShapeAndPreservesAmounts(t *testing.T) {
	svc := &fakeService{}
	input := `{"expenses":[{"amount":"10.50","description":"one","account":null,"category":null,"recurring_expense":null,"currency":"USD"}]}`
	code, _, stderr := run(t, svc, input, "transactions", "bulk-create", "--household", "hh_1", "--file", "-", "--json")
	if code != 0 {
		t.Fatalf("code=%d err=%s", code, stderr)
	}
	if svc.bulk == nil || len(svc.bulk.Expenses) != 1 || svc.bulk.Expenses[0].Amount != "10.50" {
		t.Fatalf("body=%#v", svc.bulk)
	}
}

func TestBulkDeleteRequiresExplicitConfirmation(t *testing.T) {
	svc := &fakeService{}
	code, _, _ := run(t, svc, "", "transactions", "delete-all", "--household", "hh_1", "--json")
	if code != int(cli.ExitUsage) || svc.bulkDeleted {
		t.Fatalf("code=%d deleted=%v", code, svc.bulkDeleted)
	}
	code, _, _ = run(t, svc, "", "transactions", "delete-all", "--household", "hh_1", "--yes", "--json")
	if code != 0 || !svc.bulkDeleted {
		t.Fatalf("code=%d deleted=%v", code, svc.bulkDeleted)
	}
}

func TestTransactionPatchPreservesOmittedAndExplicitNull(t *testing.T) {
	svc := &fakeService{}
	code, _, stderr := run(t, svc, "", "transactions", "update", "exp_1", "--household", "hh_1", "--description", "Changed", "--clear-account", "--clear-tags", "--type", "income", "--json")
	if code != 0 {
		t.Fatalf("code=%d err=%s", code, stderr)
	}
	if len(svc.updated) != 4 || svc.updated["account"] != nil {
		t.Fatalf("payload=%#v", svc.updated)
	}
	if _, exists := svc.updated["category"]; exists {
		t.Fatalf("omitted category was sent: %#v", svc.updated)
	}
	if tags, ok := svc.updated["tag_ids"].([]string); !ok || len(tags) != 0 {
		t.Fatalf("tags=%#v", svc.updated["tag_ids"])
	}
}

func TestBulkUpdateRequiresConfirmationAndKeepsContractBody(t *testing.T) {
	svc := &fakeService{}
	input := `{"ids":["exp_1"],"updates":{"category":null,"account":"acc_2"}}`
	code, _, _ := run(t, svc, input, "transactions", "bulk-update", "--household", "hh_1", "--file", "-", "--json")
	if code != int(cli.ExitUsage) || svc.bulkUpdated != nil {
		t.Fatalf("code=%d body=%#v", code, svc.bulkUpdated)
	}
	code, _, stderr := run(t, svc, input, "transactions", "bulk-update", "--household", "hh_1", "--file", "-", "--yes", "--json")
	if code != 0 || svc.bulkUpdated == nil {
		t.Fatalf("code=%d body=%#v err=%s", code, svc.bulkUpdated, stderr)
	}
	if !svc.bulkUpdated.Updates.Account.Present || svc.bulkUpdated.Updates.Account.Value == nil || *svc.bulkUpdated.Updates.Account.Value != "acc_2" {
		t.Fatalf("account=%#v", svc.bulkUpdated.Updates.Account)
	}
	if !svc.bulkUpdated.Updates.Category.Present || svc.bulkUpdated.Updates.Category.Value != nil {
		t.Fatalf("category=%#v", svc.bulkUpdated.Updates.Category)
	}
}

func TestBulkUpdateRejectsUnknownAndInvalidNestedFields(t *testing.T) {
	for _, input := range []string{
		`{"ids":["exp_1"],"updates":{"description":"changed"}}`,
		`{"ids":["exp_1"],"updates":{"account":42}}`,
		`{"ids":["exp_1"],"updates":{"category":""}}`,
	} {
		svc := &fakeService{}
		code, _, stderr := run(t, svc, input, "transactions", "bulk-update", "--household", "hh_1", "--file", "-", "--yes", "--json")
		if code != int(cli.ExitUsage) || svc.bulkUpdated != nil {
			t.Fatalf("input=%s code=%d body=%#v err=%s", input, code, svc.bulkUpdated, stderr)
		}
	}
}

func TestInstallmentPatchOnlySendsChangedAndClearedFields(t *testing.T) {
	svc := &fakeService{}
	code, _, stderr := run(t, svc, "", "installments", "update", "ipl_1", "--household", "hh_1", "--amount", "600.00", "--clear-category", "--json")
	if code != 0 {
		t.Fatalf("code=%d err=%s", code, stderr)
	}
	if len(svc.installmentUpdated) != 2 || svc.installmentUpdated["category"] != nil {
		t.Fatalf("payload=%#v", svc.installmentUpdated)
	}
	if _, exists := svc.installmentUpdated["account"]; exists {
		t.Fatalf("omitted account was sent: %#v", svc.installmentUpdated)
	}
}

func TestTransactionAcceptsContractCurrencyBeyondLegacyThree(t *testing.T) {
	svc := &fakeService{}
	code, _, stderr := run(t, svc, "", "transactions", "create", "--household", "hh_1", "--amount", "1.00", "--description", "Coffee", "--currency", "BRL", "--json")
	if code != 0 {
		t.Fatalf("code=%d err=%s", code, stderr)
	}
	code, _, _ = run(t, svc, "", "transactions", "create", "--household", "hh_1", "--amount", "1.00", "--description", "Coffee", "--currency", "ZZZ", "--json")
	if code != int(cli.ExitUsage) {
		t.Fatalf("code=%d", code)
	}
}

func TestConvertAndUnlinkRequireConfirmation(t *testing.T) {
	svc := &fakeService{}
	code, _, _ := run(t, svc, "", "transfers", "convert", "exp_1", "--household", "hh_1", "--to-account", "acc_2", "--json")
	if code != int(cli.ExitUsage) || svc.converted {
		t.Fatalf("convert code=%d called=%v", code, svc.converted)
	}
	code, _, _ = run(t, svc, "", "transfers", "convert", "exp_1", "--household", "hh_1", "--to-account", "acc_2", "--yes", "--json")
	if code != 0 || !svc.converted {
		t.Fatalf("convert code=%d called=%v", code, svc.converted)
	}
	code, _, _ = run(t, svc, "", "transfers", "unlink", "exp_1", "--household", "hh_1", "--json")
	if code != int(cli.ExitUsage) || svc.unlinked {
		t.Fatalf("unlink code=%d called=%v", code, svc.unlinked)
	}
	code, _, _ = run(t, svc, "", "transfers", "unlink", "exp_1", "--household", "hh_1", "--yes", "--json")
	if code != 0 || !svc.unlinked {
		t.Fatalf("unlink code=%d called=%v", code, svc.unlinked)
	}
}

func TestSpanishDestructivePromptAndStableJSONError(t *testing.T) {
	svc := &fakeService{}
	var out, errOut bytes.Buffer
	app := cli.New(cli.WithIO(strings.NewReader("sí\n"), &out, &errOut), cli.WithInteractiveDetector(func(io.Reader, io.Writer) bool { return true }), cli.WithModule(New(func(context.Context) (Service, error) { return svc, nil })))
	if code := app.Run([]string{"transactions", "delete", "exp_1", "--household", "hh_1", "--language", "es"}); code != 0 || !svc.deleted {
		t.Fatalf("code=%d deleted=%v err=%s", code, svc.deleted, errOut.String())
	}
	if !strings.Contains(errOut.String(), "Eliminar la transacción exp_1") {
		t.Fatalf("prompt=%q", errOut.String())
	}
	out.Reset()
	errOut.Reset()
	svc = &fakeService{}
	app = cli.New(cli.WithIO(strings.NewReader(""), &out, &errOut), cli.WithInteractiveDetector(func(io.Reader, io.Writer) bool { return false }), cli.WithModule(New(func(context.Context) (Service, error) { return svc, nil })))
	if code := app.Run([]string{"transactions", "create", "--household", "hh_1", "--amount", "1.00", "--description", "x", "--currency", "ZZZ", "--language", "es", "--json"}); code != int(cli.ExitUsage) {
		t.Fatalf("code=%d", code)
	}
	if !strings.Contains(errOut.String(), `"code":"invalid_currency"`) || strings.Contains(errOut.String(), "moneda") {
		t.Fatalf("json error=%q", errOut.String())
	}
}

func TestSpanishTransactionHelpAndHumanValidationError(t *testing.T) {
	svc := &fakeService{}
	code, out, stderr := run(t, svc, "", "transactions", "create", "--language", "es", "--help")
	if code != 0 {
		t.Fatalf("code=%d err=%s", code, stderr)
	}
	for _, translated := range []string{"Crea una transacción", "ID del hogar", "importe decimal", "descripción"} {
		if !strings.Contains(out, translated) {
			t.Fatalf("help missing %q: %s", translated, out)
		}
	}
	code, out, stderr = run(t, svc, "", "transactions", "list", "--language", "es", "--help")
	if code != 0 {
		t.Fatalf("list help code=%d err=%s", code, stderr)
	}
	for _, translated := range []string{"mes 1-12", "año de cuatro dígitos", "código de moneda de la transacción", "número de página", "resultados por página", "obtiene todas las transacciones coincidentes sin paginación"} {
		if !strings.Contains(out, translated) {
			t.Fatalf("list help missing %q: %s", translated, out)
		}
	}
	code, out, stderr = run(t, svc, "", "installments", "list", "--language", "es", "--help")
	if code != 0 || !strings.Contains(out, "Lista planes de cuotas") || !strings.Contains(out, "ID del hogar") {
		t.Fatalf("installments help code=%d out=%s err=%s", code, out, stderr)
	}

	code, _, stderr = run(t, svc, "", "transactions", "create", "--household", "hh_1", "--amount", "invalid", "--description", "x", "--language", "es")
	if code != int(cli.ExitUsage) || !strings.Contains(stderr, "el importe debe ser un decimal") {
		t.Fatalf("code=%d err=%s", code, stderr)
	}
}
