package contract_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	daikuv1 "github.com/DaikuFi/daiku-cli/generated/daikuv1"
)

func TestGeneratedClientDecodesExchangeRateFixture(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/v1/exchange-rates/" {
			http.Error(w, "unexpected request", http.StatusBadRequest)
			return
		}
		if got := r.Header.Get("Authorization"); got != "Bearer fixture-token" {
			http.Error(w, "missing bearer token", http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `[{"from_currency":"USD","to_currency":"UYU","rate":"40.125"}]`)
	}))
	t.Cleanup(server.Close)

	client, err := daikuv1.NewClientWithResponses(server.URL, daikuv1.WithRequestEditorFn(
		func(_ context.Context, req *http.Request) error {
			req.Header.Set("Authorization", "Bearer fixture-token")
			return nil
		},
	))
	if err != nil {
		t.Fatal(err)
	}

	response, err := client.DaikuExchangeRatesGetWithResponse(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode() != http.StatusOK || response.JSON200 == nil {
		t.Fatalf("response = status %d body %s", response.StatusCode(), response.Body)
	}
	rates := *response.JSON200
	if len(rates) != 1 || rates[0].Rate != "40.125" || rates[0].FromCurrency != "USD" {
		t.Fatalf("decoded rates = %#v", rates)
	}
}

func TestGeneratedClientDecodesEnrichedExpenseFixture(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/v1/households/hsh_fixture/expenses/" {
			http.Error(w, "unexpected request", http.StatusBadRequest)
			return
		}
		if got := r.Header.Get("Authorization"); got != "Bearer fixture-token" {
			http.Error(w, "missing bearer token", http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `[{
			"id":"exp_fixture",
			"amount":"100.00",
			"description":"Transferencia",
			"currency":"BRL",
			"transaction_type":"transfer",
			"installment_total":12,
			"transfer_peer":{
				"account":"acc_peer",
				"account_name":"Conta",
				"amount":"525.50",
				"currency":"BRL"
			},
			"cash_flow_link":{
				"id":"cfl_fixture",
				"side":"cash_out"
			}
		}]`)
	}))
	t.Cleanup(server.Close)

	client, err := daikuv1.NewClientWithResponses(server.URL, daikuv1.WithRequestEditorFn(
		func(_ context.Context, req *http.Request) error {
			req.Header.Set("Authorization", "Bearer fixture-token")
			return nil
		},
	))
	if err != nil {
		t.Fatal(err)
	}

	response, err := client.DaikuHouseholdsHouseholdPkExpensesGetWithResponse(
		context.Background(), "hsh_fixture", nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode() != http.StatusOK || response.JSON200 == nil {
		t.Fatalf("response = status %d body %s", response.StatusCode(), response.Body)
	}
	expenses, err := response.JSON200.AsExpenseListResponse0()
	if err != nil {
		t.Fatal(err)
	}
	if len(expenses) != 1 {
		t.Fatalf("decoded expenses = %#v", expenses)
	}
	expense := expenses[0]
	if expense.InstallmentTotal == nil || *expense.InstallmentTotal != 12 {
		t.Fatalf("installment_total = %#v", expense.InstallmentTotal)
	}
	if expense.TransferPeer == nil ||
		expense.TransferPeer.Account != "acc_peer" ||
		expense.TransferPeer.Currency != daikuv1.Currency3e8EnumBRL {
		t.Fatalf("transfer_peer = %#v", expense.TransferPeer)
	}
	if expense.CashFlowLink == nil ||
		expense.CashFlowLink.Id != "cfl_fixture" ||
		expense.CashFlowLink.Side != daikuv1.CashFlowLinkSummarySideEnumCashOut {
		t.Fatalf("cash_flow_link = %#v", expense.CashFlowLink)
	}
	if expense.TransactionType == nil || *expense.TransactionType != daikuv1.ExpenseTransactionTypeEnumTransfer {
		t.Fatalf("transaction_type = %#v", expense.TransactionType)
	}
}
