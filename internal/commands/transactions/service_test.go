package transactions

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/DaikuFi/daiku-cli/generated/daikuv1"
	"github.com/DaikuFi/daiku-cli/internal/cli"
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

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) Do(request *http.Request) (*http.Response, error) { return f(request) }

func TestResponseErrorMapsRoleRejectionToForbidden(t *testing.T) {
	err := responseError(http.StatusForbidden, []byte(`{"detail":"editor role required","code":"permission_denied"}`))
	var cliErr *cli.Error
	if !errors.As(err, &cliErr) || cliErr.ExitCode != cli.ExitForbidden {
		t.Fatalf("error = %#v", err)
	}
}
