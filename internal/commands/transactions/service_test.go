package transactions

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/DaikuFi/daiku-cli/generated/daikuv1"
	authcore "github.com/DaikuFi/daiku-cli/internal/auth"
	"github.com/DaikuFi/daiku-cli/internal/cli"
	"github.com/DaikuFi/daiku-cli/internal/credentials"
	"github.com/DaikuFi/daiku-cli/internal/profiles"
)

func TestGeneratedServiceSendsTransferLegsWithoutRecalculation(t *testing.T) {
	var request map[string]any
	doer := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/v1/households/hh_1/transfers/" {
			t.Fatalf("request = %s %s", r.Method, r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatal(err)
		}
		return &http.Response{StatusCode: http.StatusCreated, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(`{"transfer_group":"trg_1","transactions":[]}`))}, nil
	})
	client, err := daikuv1.NewClientWithResponses("https://api.daiku.test", daikuv1.WithHTTPClient(doer))
	if err != nil {
		t.Fatal(err)
	}
	toAmount := "2.50"
	_, err = (generatedService{client}).CreateTransfer(context.Background(), "hh_1", daikuv1.TransferCreateRequestRequest{
		Amount: "100.00", FromAccount: "acc_a", ToAccount: "acc_b", ToAmount: &toAmount,
	})
	if err != nil {
		t.Fatal(err)
	}
	if request["amount"] != "100.00" || request["to_amount"] != "2.50" {
		t.Fatalf("amounts changed: %#v", request)
	}
	if _, exists := request["balance"]; exists {
		t.Fatalf("CLI must not send computed balances: %#v", request)
	}
}

func TestGeneratedServiceListDecodesEnrichedExpensePageNatively(t *testing.T) {
	doer := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/v1/households/hh_1/expenses/" {
			t.Fatalf("request = %s %s", r.Method, r.URL.Path)
		}
		body := `{
			"count":1,
			"next":null,
			"previous":null,
			"results":[{
				"id":"exp_1",
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
					"id":"cfl_1",
					"side":"cash_out"
				}
			}]
		}`
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(body)),
		}, nil
	})
	client, err := daikuv1.NewClientWithResponses("https://api.daiku.test", daikuv1.WithHTTPClient(doer))
	if err != nil {
		t.Fatal(err)
	}

	result, err := (generatedService{client}).List(context.Background(), "hh_1", nil)
	if err != nil {
		t.Fatal(err)
	}
	page, ok := result.(daikuv1.ExpensePage)
	if !ok || len(page.Results) != 1 {
		t.Fatalf("result = %#v", result)
	}
	expense := page.Results[0]
	if expense.InstallmentTotal == nil || *expense.InstallmentTotal != 12 {
		t.Fatalf("installment_total = %#v", expense.InstallmentTotal)
	}
	if expense.TransferPeer == nil ||
		expense.TransferPeer.Account != "acc_peer" ||
		expense.TransferPeer.AccountName != "Conta" ||
		expense.TransferPeer.Amount != "525.50" ||
		expense.TransferPeer.Currency != daikuv1.Currency3e8EnumBRL {
		t.Fatalf("transfer_peer = %#v", expense.TransferPeer)
	}
	if expense.CashFlowLink == nil ||
		expense.CashFlowLink.Id != "cfl_1" ||
		expense.CashFlowLink.Side != daikuv1.CashFlowLinkSummarySideEnumCashOut {
		t.Fatalf("cash_flow_link = %#v", expense.CashFlowLink)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) Do(request *http.Request) (*http.Response, error)        { return f(request) }
func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) { return f(request) }

func TestResponseErrorMapsRoleRejectionToForbidden(t *testing.T) {
	err := responseError(http.StatusForbidden, []byte(`{"detail":"editor role required","code":"permission_denied"}`))
	var cliErr *cli.Error
	if !errors.As(err, &cliErr) || cliErr.ExitCode != cli.ExitForbidden {
		t.Fatalf("error = %#v", err)
	}
	if cliErr.Message != "Daiku API returned HTTP 403: editor role required" {
		t.Fatalf("message = %q", cliErr.Message)
	}
}

func TestResponseErrorOnlyIncludesMeaningfulPublicError(t *testing.T) {
	err := responseError(http.StatusNotFound, []byte(`{"error":{"errors":null,"message":"","status_code":0}}`))
	var cliErr *cli.Error
	if !errors.As(err, &cliErr) {
		t.Fatal(err)
	}
	if cliErr.Message != "Daiku API returned HTTP 404" {
		t.Fatalf("message = %q", cliErr.Message)
	}

	err = responseError(http.StatusBadRequest, []byte(`{"error":{"errors":{"amount":["invalid"]},"message":"Invalid request.","status_code":400}}`))
	if !errors.As(err, &cliErr) || !strings.Contains(cliErr.Message, `"message":"Invalid request."`) {
		t.Fatalf("error = %#v", err)
	}
}

type memoryCredentials struct{ token credentials.Token }

func (m *memoryCredentials) Get(string) (credentials.Token, error)       { return m.token, nil }
func (m *memoryCredentials) Put(_ string, token credentials.Token) error { m.token = token; return nil }
func (*memoryCredentials) Delete(string) error                           { return nil }

func TestGeneratedFactoryRefreshesExpiredAccessToken(t *testing.T) {
	store := profiles.Store{Path: filepath.Join(t.TempDir(), "config.json")}
	if err := os.Chmod(filepath.Dir(store.Path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := store.Save(profiles.Config{Current: "work", Profiles: map[string]profiles.Profile{"work": {APIURL: "https://api.daiku.test/api/v1/"}}}); err != nil {
		t.Fatal(err)
	}
	creds := &memoryCredentials{token: credentials.Token{AccessToken: "expired", RefreshToken: "refresh", ExpiresAt: time.Now().Add(-time.Minute).Unix()}}
	doer := roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.Path != "/oauth/token/" {
			t.Fatalf("path=%s", request.URL.Path)
		}
		return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(`{"access_token":"fresh","refresh_token":"rotated","expires_in":3600,"scope":"finance:read finance:write"}`))}, nil
	})
	oauth, err := authcore.New(authcore.Config{ClientID: "daiku-cli", AuthorizeURL: "https://auth.daiku.test/oauth/authorize/", TokenURL: "https://auth.daiku.test/oauth/token/", RevokeURL: "https://auth.daiku.test/oauth/revoke/", HTTPClient: &http.Client{Transport: doer}})
	if err != nil {
		t.Fatal(err)
	}
	manager := &authcore.Manager{Store: creds, OAuth: oauth, DisableProcessLock: true}
	if _, err = GeneratedServiceFactory(store, manager)(context.Background()); err != nil {
		t.Fatal(err)
	}
	if creds.token.AccessToken != "fresh" || creds.token.RefreshToken != "rotated" {
		t.Fatalf("token=%#v", creds.token)
	}
}

func TestGeneratedPatchPreservesOmittedAndNullOnWire(t *testing.T) {
	var payload map[string]any
	doer := roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.Method != http.MethodPatch {
			t.Fatalf("method=%s", request.Method)
		}
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(`{"amount":"1.00","description":"changed","account":null,"category":null,"created_by":null,"installment_number":null,"installment_plan":null,"recurring_expense":null,"transfer_group":null}`))}, nil
	})
	client, err := daikuv1.NewClientWithResponses("https://api.daiku.test", daikuv1.WithHTTPClient(doer))
	if err != nil {
		t.Fatal(err)
	}
	_, err = (generatedService{client}).Update(context.Background(), "hh_1", "exp_1", PatchBody{"description": "changed", "account": nil})
	if err != nil {
		t.Fatal(err)
	}
	if len(payload) != 2 || payload["account"] != nil {
		t.Fatalf("payload=%#v", payload)
	}
	if _, ok := payload["category"]; ok {
		t.Fatalf("omitted category sent: %#v", payload)
	}
}

func TestGeneratedBulkUpdatePreservesTypedOmittedAndNullOnWire(t *testing.T) {
	var payload map[string]any
	doer := roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.Method != http.MethodPatch {
			t.Fatalf("method=%s", request.Method)
		}
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(`{"updated":1,"skipped":0}`))}, nil
	})
	client, err := daikuv1.NewClientWithResponses("https://api.daiku.test", daikuv1.WithHTTPClient(doer))
	if err != nil {
		t.Fatal(err)
	}
	account := "acc_2"
	_, err = (generatedService{client}).BulkUpdate(context.Background(), "hh_1", BulkUpdateBody{
		IDs: []string{"exp_1"},
		Updates: BulkUpdateFields{
			Account:  OptionalNullableString{Present: true, Value: &account},
			Category: OptionalNullableString{Present: true},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	updates, ok := payload["updates"].(map[string]any)
	if !ok || len(updates) != 2 || updates["account"] != "acc_2" || updates["category"] != nil {
		t.Fatalf("payload=%#v", payload)
	}
}
