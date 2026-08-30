package transactions

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/DaikuFi/daiku-cli/generated/daikuv1"
	authcore "github.com/DaikuFi/daiku-cli/internal/auth"
	"github.com/DaikuFi/daiku-cli/internal/profiles"
)

type Service interface {
	List(context.Context, string, *daikuv1.DaikuHouseholdsHouseholdPkExpensesGetParams) (any, error)
	Create(context.Context, string, daikuv1.ExpenseRequest) (any, error)
	Update(context.Context, string, string, PatchBody) (any, error)
	Delete(context.Context, string, string, *daikuv1.DaikuHouseholdsHouseholdPkExpensesIdDeleteParams) error
	BulkCreate(context.Context, string, daikuv1.ExpenseBulkCreateRequestRequest) (any, error)
	BulkUpdate(context.Context, string, BulkUpdateBody) (any, error)
	BulkDelete(context.Context, string) (any, error)
	CreateTransfer(context.Context, string, daikuv1.TransferCreateRequestRequest) (any, error)
	ConvertTransfer(context.Context, string, string, daikuv1.TransferConvertRequestRequest) (any, error)
	TransferCandidates(context.Context, string, string) (any, error)
	UnlinkTransfer(context.Context, string, string) (any, error)
	CreateInstallments(context.Context, string, daikuv1.InstallmentCreateRequestRequest) (any, error)
	GetInstallment(context.Context, string, string) (any, error)
	UpdateInstallment(context.Context, string, string, PatchBody) (any, error)
}

type PatchBody map[string]any
type BulkUpdateBody struct {
	IDs     []string  `json:"ids"`
	Updates PatchBody `json:"updates"`
}

type ServiceFactory func(context.Context) (Service, error)

func GeneratedServiceFactory(profileStore profiles.Store, manager *authcore.Manager) ServiceFactory {
	return func(ctx context.Context) (Service, error) {
		config, err := profileStore.Load()
		if err != nil {
			return nil, safe("config_error", "profile configuration could not be read")
		}
		profile, ok := config.Profiles[config.Current]
		if config.Current == "" || !ok {
			return nil, safe("profile_required", "select a profile before using transactions")
		}
		if manager == nil {
			return nil, safe("client_error", "authentication manager is not configured")
		}
		token, err := manager.AccessToken(ctx, config.Current)
		if err != nil {
			return nil, safe("authentication_required", "authenticate this profile before using transactions")
		}
		apiURL, err := profiles.NormalizeAPIURL(profile.APIURL)
		if err != nil {
			return nil, safe("config_error", "profile configuration contains an invalid API URL")
		}
		server := strings.TrimSuffix(apiURL, "/api/v1/")
		client, err := daikuv1.NewClientWithResponses(server, daikuv1.WithRequestEditorFn(func(_ context.Context, request *http.Request) error {
			request.Header.Set("Authorization", "Bearer "+token)
			return nil
		}))
		if err != nil {
			return nil, safe("client_error", "the Daiku API client could not be created")
		}
		return generatedService{client}, nil
	}
}

type generatedService struct{ client *daikuv1.ClientWithResponses }

func responseError(status int, body []byte) error {
	var public daikuv1.PublicError
	_ = json.Unmarshal(body, &public)
	message := fmt.Sprintf("Daiku API returned HTTP %d", status)
	if encoded, err := json.Marshal(public); err == nil && string(encoded) != "{}" {
		message += ": " + string(encoded)
	}
	return safeStatus(status, message)
}

func (s generatedService) List(ctx context.Context, hh string, p *daikuv1.DaikuHouseholdsHouseholdPkExpensesGetParams) (any, error) {
	r, err := s.client.DaikuHouseholdsHouseholdPkExpensesGetWithResponse(ctx, hh, p)
	if err != nil {
		return nil, err
	}
	if r.JSON200 == nil {
		return nil, responseError(r.StatusCode(), r.Body)
	}
	if page, err := r.JSON200.AsExpensePage(); err == nil {
		return page, nil
	}
	items, err := r.JSON200.AsExpenseListResponse0()
	if err != nil {
		return nil, safe("invalid_response", "the Daiku API returned an invalid transaction list")
	}
	return items, nil
}
func (s generatedService) Create(ctx context.Context, hh string, b daikuv1.ExpenseRequest) (any, error) {
	r, e := s.client.DaikuHouseholdsHouseholdPkExpensesPostWithResponse(ctx, hh, b)
	if e != nil {
		return nil, e
	}
	if r.JSON201 == nil {
		return nil, responseError(r.StatusCode(), r.Body)
	}
	return *r.JSON201, nil
}
func (s generatedService) Update(ctx context.Context, hh, id string, b PatchBody) (any, error) {
	payload, err := json.Marshal(b)
	if err != nil {
		return nil, safe("invalid_request", "transaction update could not be encoded")
	}
	r, e := s.client.DaikuHouseholdsHouseholdPkExpensesIdPatchWithBodyWithResponse(ctx, hh, id, "application/json", strings.NewReader(string(payload)))
	if e != nil {
		return nil, e
	}
	if r.JSON200 == nil {
		return nil, responseError(r.StatusCode(), r.Body)
	}
	return *r.JSON200, nil
}
func (s generatedService) Delete(ctx context.Context, hh, id string, p *daikuv1.DaikuHouseholdsHouseholdPkExpensesIdDeleteParams) error {
	r, e := s.client.DaikuHouseholdsHouseholdPkExpensesIdDeleteWithResponse(ctx, hh, id, p)
	if e != nil {
		return e
	}
	if r.StatusCode() != 204 {
		return responseError(r.StatusCode(), r.Body)
	}
	return nil
}
func (s generatedService) BulkCreate(ctx context.Context, hh string, b daikuv1.ExpenseBulkCreateRequestRequest) (any, error) {
	r, e := s.client.DaikuHouseholdsHouseholdPkExpensesBulkPostWithResponse(ctx, hh, b)
	if e != nil {
		return nil, e
	}
	if r.JSON201 == nil {
		return nil, responseError(r.StatusCode(), r.Body)
	}
	return *r.JSON201, nil
}
func (s generatedService) BulkUpdate(ctx context.Context, hh string, b BulkUpdateBody) (any, error) {
	payload, err := json.Marshal(b)
	if err != nil {
		return nil, safe("invalid_request", "bulk update could not be encoded")
	}
	r, e := s.client.DaikuHouseholdsHouseholdPkExpensesBulkPatchWithBodyWithResponse(ctx, hh, "application/json", strings.NewReader(string(payload)))
	if e != nil {
		return nil, e
	}
	if r.JSON200 == nil {
		return nil, responseError(r.StatusCode(), r.Body)
	}
	return *r.JSON200, nil
}
func (s generatedService) BulkDelete(ctx context.Context, hh string) (any, error) {
	r, err := s.client.DaikuHouseholdsHouseholdPkExpensesDeleteWithResponse(ctx, hh)
	if err != nil {
		return nil, err
	}
	if r.JSON200 == nil {
		return nil, responseError(r.StatusCode(), r.Body)
	}
	return *r.JSON200, nil
}
func (s generatedService) CreateTransfer(ctx context.Context, hh string, b daikuv1.TransferCreateRequestRequest) (any, error) {
	r, e := s.client.DaikuHouseholdsHouseholdPkTransfersPostWithResponse(ctx, hh, b)
	if e != nil {
		return nil, e
	}
	if r.JSON201 == nil {
		return nil, responseError(r.StatusCode(), r.Body)
	}
	return *r.JSON201, nil
}
func (s generatedService) ConvertTransfer(ctx context.Context, hh, id string, b daikuv1.TransferConvertRequestRequest) (any, error) {
	r, e := s.client.DaikuHouseholdsHouseholdPkExpensesIdConvertToTransferPostWithResponse(ctx, hh, id, b)
	if e != nil {
		return nil, e
	}
	if r.JSON200 == nil {
		return nil, responseError(r.StatusCode(), r.Body)
	}
	return *r.JSON200, nil
}
func (s generatedService) TransferCandidates(ctx context.Context, hh, id string) (any, error) {
	r, e := s.client.DaikuHouseholdsHouseholdPkExpensesIdTransferCandidatesGetWithResponse(ctx, hh, id)
	if e != nil {
		return nil, e
	}
	if r.JSON200 == nil {
		return nil, responseError(r.StatusCode(), r.Body)
	}
	return *r.JSON200, nil
}
func (s generatedService) UnlinkTransfer(ctx context.Context, hh, id string) (any, error) {
	r, e := s.client.DaikuHouseholdsHouseholdPkExpensesIdUnlinkTransferPostWithResponse(ctx, hh, id)
	if e != nil {
		return nil, e
	}
	if r.JSON200 == nil {
		return nil, responseError(r.StatusCode(), r.Body)
	}
	return *r.JSON200, nil
}
func (s generatedService) CreateInstallments(ctx context.Context, hh string, b daikuv1.InstallmentCreateRequestRequest) (any, error) {
	r, e := s.client.DaikuHouseholdsHouseholdPkExpensesInstallmentsPostWithResponse(ctx, hh, b)
	if e != nil {
		return nil, e
	}
	if r.JSON201 == nil {
		return nil, responseError(r.StatusCode(), r.Body)
	}
	return *r.JSON201, nil
}
func (s generatedService) GetInstallment(ctx context.Context, hh, id string) (any, error) {
	r, e := s.client.DaikuHouseholdsHouseholdPkInstallmentPlansIdGetWithResponse(ctx, hh, id)
	if e != nil {
		return nil, e
	}
	if r.JSON200 == nil {
		return nil, responseError(r.StatusCode(), r.Body)
	}
	return *r.JSON200, nil
}
func (s generatedService) UpdateInstallment(ctx context.Context, hh, id string, b PatchBody) (any, error) {
	payload, err := json.Marshal(b)
	if err != nil {
		return nil, safe("invalid_request", "installment update could not be encoded")
	}
	r, e := s.client.DaikuHouseholdsHouseholdPkInstallmentPlansIdPatchWithBodyWithResponse(ctx, hh, id, "application/json", strings.NewReader(string(payload)))
	if e != nil {
		return nil, e
	}
	if r.JSON200 == nil {
		return nil, responseError(r.StatusCode(), r.Body)
	}
	return *r.JSON200, nil
}
