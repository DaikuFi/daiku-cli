package transactions

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"testing"

	"github.com/DaikuFi/daiku-cli/generated/daikuv1"
	"github.com/DaikuFi/daiku-cli/internal/cli"
)

type fakeService struct {
	created     *daikuv1.ExpenseRequest
	transfer    *daikuv1.TransferCreateRequestRequest
	installment *daikuv1.InstallmentCreateRequestRequest
	bulk        *daikuv1.ExpenseBulkCreateRequestRequest
	deleted     bool
	bulkDeleted bool
	listed      *daikuv1.DaikuHouseholdsHouseholdPkExpensesGetParams
}

func (f *fakeService) List(_ context.Context, _ string, p *daikuv1.DaikuHouseholdsHouseholdPkExpensesGetParams) (any, error) {
	f.listed = p
	return []daikuv1.Expense{}, nil
}
func (f *fakeService) Create(_ context.Context, _ string, b daikuv1.ExpenseRequest) (any, error) {
	f.created = &b
	return daikuv1.Expense{Amount: b.Amount, Description: b.Description}, nil
}
func (*fakeService) Update(context.Context, string, string, daikuv1.PatchedExpenseRequest) (any, error) {
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
func (*fakeService) BulkUpdate(context.Context, string, daikuv1.PatchedExpenseBulkUpdateRequestRequest) (any, error) {
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
func (*fakeService) ConvertTransfer(context.Context, string, string, daikuv1.TransferConvertRequestRequest) (any, error) {
	return daikuv1.TransferResponse{}, nil
}
func (*fakeService) TransferCandidates(context.Context, string, string) (any, error) {
	return []daikuv1.Expense{}, nil
}
func (*fakeService) UnlinkTransfer(context.Context, string, string) (any, error) {
	return daikuv1.TransferUnlinkResponse{}, nil
}
func (f *fakeService) CreateInstallments(_ context.Context, _ string, body daikuv1.InstallmentCreateRequestRequest) (any, error) {
	f.installment = &body
	return daikuv1.InstallmentPlan{}, nil
}
func (*fakeService) GetInstallment(context.Context, string, string) (any, error) {
	return daikuv1.InstallmentPlan{}, nil
}
func (*fakeService) UpdateInstallment(context.Context, string, string, daikuv1.PatchedInstallmentPlanUpdateRequest) (any, error) {
	return daikuv1.InstallmentPlan{}, nil
}

func run(t *testing.T, svc *fakeService, input string, args ...string) (int, string, string) {
	t.Helper()
	var out, errOut bytes.Buffer
	app := cli.New(cli.WithIO(bytes.NewBufferString(input), &out, &errOut), cli.WithInteractiveDetector(func(_ io.Reader, _ io.Writer) bool { return false }), cli.WithModule(New(func() (Service, error) { return svc, nil })))
	code := app.Run(args)
	return code, out.String(), errOut.String()
}

func TestCreatePreservesDecimalStringAndJSONEnvelope(t *testing.T) {
	svc := &fakeService{}
	code, out, stderr := run(t, svc, "", "transactions", "create", "--household", "hh_1", "--amount", "001.2300", "--description", "Lunch", "--currency", "USD", "--json")
	if code != int(cli.ExitUsage) {
		t.Fatalf("code=%d out=%s err=%s", code, out, stderr)
	}
	code, out, stderr = run(t, svc, "", "transactions", "create", "--household", "hh_1", "--amount", "1.2300", "--description", "Lunch", "--currency", "USD", "--json")
	if code != 0 {
		t.Fatalf("code=%d err=%s", code, stderr)
	}
	if svc.created == nil || svc.created.Amount != "1.2300" {
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
	code, _, stderr := run(t, svc, "", "transfers", "create", "--household", "hh_1", "--from-account", "acc_a", "--to-account", "acc_b", "--amount", "100.00", "--to-amount", "2.500", "--json")
	if code != 0 {
		t.Fatalf("code=%d err=%s", code, stderr)
	}
	if svc.transfer == nil || svc.transfer.Amount != "100.00" || svc.transfer.ToAmount == nil || *svc.transfer.ToAmount != "2.500" {
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
	code, _, stderr := run(t, svc, "", "installments", "create", "--household", "hh_1", "--amount", "1200.00", "--description", "Laptop", "--currency", "UYU", "--date", "2026-08-29", "--count", "12", "--json")
	if code != 0 {
		t.Fatalf("code=%d err=%s", code, stderr)
	}
	if svc.installment == nil || svc.installment.Amount != "1200.00" || svc.installment.Installments != 12 {
		t.Fatalf("body=%#v", svc.installment)
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
