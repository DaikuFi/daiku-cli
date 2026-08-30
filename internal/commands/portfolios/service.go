package portfolios

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	daikuv1 "github.com/DaikuFi/daiku-cli/generated/daikuv1"
	"github.com/DaikuFi/daiku-cli/internal/cli"
	"github.com/DaikuFi/daiku-cli/internal/profiles"
)

type tokenSource interface {
	AccessToken(context.Context, string) (string, error)
}

type Service interface {
	PortfolioList(context.Context) ([]daikuv1.PortfolioList, error)
	PortfolioGet(context.Context, string) (*daikuv1.PublicPortfolio, error)
	PortfolioCreate(context.Context, daikuv1.PortfolioListRequest) (*daikuv1.PortfolioList, error)
	PortfolioUpdate(context.Context, string, daikuv1.PatchedPortfolioListRequest) (*daikuv1.PortfolioList, error)
	PortfolioDelete(context.Context, string) error
	Totals(context.Context, string) (*daikuv1.PortfolioTotals, error)
	Holdings(context.Context, string) (*daikuv1.PortfolioHoldings, error)
	BucketList(context.Context, string) ([]daikuv1.BucketList, error)
	BucketCreate(context.Context, string, daikuv1.BucketListRequest) (*daikuv1.BucketList, error)
	BucketUpdate(context.Context, string, string, daikuv1.PatchedBucketListRequest) (*daikuv1.BucketList, error)
	BucketDelete(context.Context, string, string) error
	AssetList(context.Context, string) ([]daikuv1.PublicAsset, error)
	AssetCreate(context.Context, string, daikuv1.PublicAssetRequest) (*daikuv1.PublicAsset, error)
	AssetUpdate(context.Context, string, string, daikuv1.PatchedPublicAssetRequest) (*daikuv1.PublicAsset, error)
	AssetDelete(context.Context, string, string) error
	CashflowList(context.Context, string) ([]daikuv1.AssetCashFlow, error)
	CashflowCreate(context.Context, string, daikuv1.AssetCashFlowRequest) (*daikuv1.AssetCashFlow, error)
	CashflowUpdate(context.Context, string, string, daikuv1.PatchedAssetCashFlowRequest) (*daikuv1.AssetCashFlow, error)
	CashflowDelete(context.Context, string, string) error
	HistoryList(context.Context, string) ([]daikuv1.AssetValueHistory, error)
	HistoryCreate(context.Context, string, daikuv1.AssetValueHistoryRequest) (*daikuv1.AssetValueHistory, error)
	HistoryUpdate(context.Context, string, string, daikuv1.PatchedAssetValueHistoryRequest) (*daikuv1.AssetValueHistory, error)
	HistoryDelete(context.Context, string, string) error
}

type Factory func(context.Context) (Service, error)

func GeneratedFactory(store profiles.Store, tokens tokenSource, httpClient *http.Client) Factory {
	return func(ctx context.Context) (Service, error) {
		cfg, err := store.Load()
		if err != nil {
			return nil, commandError("profile_error", "profile configuration could not be read", cli.ExitFailure)
		}
		profile, ok := cfg.Profiles[cfg.Current]
		if cfg.Current == "" || !ok {
			return nil, commandError("profile_required", "select a profile before using portfolios", cli.ExitAuth)
		}
		token, err := tokens.AccessToken(ctx, cfg.Current)
		if err != nil {
			return nil, commandError("authentication_required", "authenticate the active profile before using portfolios", cli.ExitAuth)
		}
		base := strings.TrimSuffix(profile.APIURL, "/api/v1/")
		opts := []daikuv1.ClientOption{daikuv1.WithRequestEditorFn(func(_ context.Context, request *http.Request) error {
			request.Header.Set("Authorization", "Bearer "+token)
			return nil
		})}
		if httpClient != nil {
			opts = append(opts, daikuv1.WithHTTPClient(httpClient))
		}
		client, err := daikuv1.NewClientWithResponses(base, opts...)
		if err != nil {
			return nil, commandError("client_error", "the Daiku API client could not be created", cli.ExitFailure)
		}
		return generatedService{client}, nil
	}
}

type generatedService struct{ c *daikuv1.ClientWithResponses }

func apiError(status int, body []byte) error {
	var public daikuv1.PublicError
	_ = json.Unmarshal(body, &public)
	message := fmt.Sprintf("Daiku API returned HTTP %d", status)
	if encoded, err := json.Marshal(public); err == nil && string(encoded) != "{}" {
		message += ": " + string(encoded)
	}
	code, exit := "api_error", cli.ExitFailure
	switch status {
	case 400:
		code, exit = "invalid_request", cli.ExitUsage
	case 401:
		code, exit = "authentication_required", cli.ExitAuth
	case 403:
		code, exit = "forbidden", cli.ExitForbidden
	case 404:
		code, exit = "not_found", cli.ExitNotFound
	case 409:
		code, exit = "conflict", cli.ExitConflict
	case 429:
		code = "rate_limited"
	}
	return commandError(code, message, exit)
}

func (s generatedService) PortfolioList(ctx context.Context) ([]daikuv1.PortfolioList, error) {
	r, e := s.c.DaikuPortfoliosGetWithResponse(ctx)
	if e != nil {
		return nil, e
	}
	if r.JSON200 == nil {
		return nil, apiError(r.StatusCode(), r.Body)
	}
	return *r.JSON200, nil
}
func (s generatedService) PortfolioGet(ctx context.Context, id string) (*daikuv1.PublicPortfolio, error) {
	r, e := s.c.DaikuPortfoliosIdGetWithResponse(ctx, id)
	if e != nil {
		return nil, e
	}
	if r.JSON200 == nil {
		return nil, apiError(r.StatusCode(), r.Body)
	}
	return r.JSON200, nil
}
func (s generatedService) PortfolioCreate(ctx context.Context, b daikuv1.PortfolioListRequest) (*daikuv1.PortfolioList, error) {
	r, e := s.c.DaikuPortfoliosPostWithResponse(ctx, b)
	if e != nil {
		return nil, e
	}
	if r.JSON201 == nil {
		return nil, apiError(r.StatusCode(), r.Body)
	}
	return r.JSON201, nil
}
func (s generatedService) PortfolioUpdate(ctx context.Context, id string, b daikuv1.PatchedPortfolioListRequest) (*daikuv1.PortfolioList, error) {
	r, e := s.c.DaikuPortfoliosIdPatchWithResponse(ctx, id, b)
	if e != nil {
		return nil, e
	}
	if r.JSON200 == nil {
		return nil, apiError(r.StatusCode(), r.Body)
	}
	return r.JSON200, nil
}
func (s generatedService) PortfolioDelete(ctx context.Context, id string) error {
	r, e := s.c.DaikuPortfoliosIdDeleteWithResponse(ctx, id)
	if e != nil {
		return e
	}
	if r.StatusCode() != 204 {
		return apiError(r.StatusCode(), r.Body)
	}
	return nil
}
func (s generatedService) Totals(ctx context.Context, id string) (*daikuv1.PortfolioTotals, error) {
	r, e := s.c.DaikuPortfoliosIdTotalsGetWithResponse(ctx, id)
	if e != nil {
		return nil, e
	}
	if r.JSON200 == nil {
		return nil, apiError(r.StatusCode(), r.Body)
	}
	return r.JSON200, nil
}
func (s generatedService) Holdings(ctx context.Context, id string) (*daikuv1.PortfolioHoldings, error) {
	r, e := s.c.DaikuPortfoliosIdHoldingsGetWithResponse(ctx, id)
	if e != nil {
		return nil, e
	}
	if r.JSON200 == nil {
		return nil, apiError(r.StatusCode(), r.Body)
	}
	return r.JSON200, nil
}
func (s generatedService) BucketList(ctx context.Context, p string) ([]daikuv1.BucketList, error) {
	r, e := s.c.DaikuPortfoliosPortfolioPkBucketsGetWithResponse(ctx, p)
	if e != nil {
		return nil, e
	}
	if r.JSON200 == nil {
		return nil, apiError(r.StatusCode(), r.Body)
	}
	return *r.JSON200, nil
}
func (s generatedService) BucketCreate(ctx context.Context, p string, b daikuv1.BucketListRequest) (*daikuv1.BucketList, error) {
	r, e := s.c.DaikuPortfoliosPortfolioPkBucketsPostWithResponse(ctx, p, b)
	if e != nil {
		return nil, e
	}
	if r.JSON201 == nil {
		return nil, apiError(r.StatusCode(), r.Body)
	}
	return r.JSON201, nil
}
func (s generatedService) BucketUpdate(ctx context.Context, p, id string, b daikuv1.PatchedBucketListRequest) (*daikuv1.BucketList, error) {
	r, e := s.c.DaikuPortfoliosPortfolioPkBucketsIdPatchWithResponse(ctx, p, id, b)
	if e != nil {
		return nil, e
	}
	if r.JSON200 == nil {
		return nil, apiError(r.StatusCode(), r.Body)
	}
	return r.JSON200, nil
}
func (s generatedService) BucketDelete(ctx context.Context, p, id string) error {
	r, e := s.c.DaikuPortfoliosPortfolioPkBucketsIdDeleteWithResponse(ctx, p, id)
	if e != nil {
		return e
	}
	if r.StatusCode() != 204 {
		return apiError(r.StatusCode(), r.Body)
	}
	return nil
}
func (s generatedService) AssetList(ctx context.Context, b string) ([]daikuv1.PublicAsset, error) {
	r, e := s.c.DaikuBucketsBucketPkAssetsGetWithResponse(ctx, b)
	if e != nil {
		return nil, e
	}
	if r.JSON200 == nil {
		return nil, apiError(r.StatusCode(), r.Body)
	}
	return *r.JSON200, nil
}
func (s generatedService) AssetCreate(ctx context.Context, b string, v daikuv1.PublicAssetRequest) (*daikuv1.PublicAsset, error) {
	r, e := s.c.DaikuBucketsBucketPkAssetsPostWithResponse(ctx, b, v)
	if e != nil {
		return nil, e
	}
	if r.JSON201 == nil {
		return nil, apiError(r.StatusCode(), r.Body)
	}
	return r.JSON201, nil
}
func (s generatedService) AssetUpdate(ctx context.Context, b, id string, v daikuv1.PatchedPublicAssetRequest) (*daikuv1.PublicAsset, error) {
	r, e := s.c.DaikuBucketsBucketPkAssetsIdPatchWithResponse(ctx, b, id, v)
	if e != nil {
		return nil, e
	}
	if r.JSON200 == nil {
		return nil, apiError(r.StatusCode(), r.Body)
	}
	return r.JSON200, nil
}
func (s generatedService) AssetDelete(ctx context.Context, b, id string) error {
	r, e := s.c.DaikuBucketsBucketPkAssetsIdDeleteWithResponse(ctx, b, id)
	if e != nil {
		return e
	}
	if r.StatusCode() != 204 {
		return apiError(r.StatusCode(), r.Body)
	}
	return nil
}
func (s generatedService) CashflowList(ctx context.Context, a string) ([]daikuv1.AssetCashFlow, error) {
	r, e := s.c.DaikuAssetsIdCashflowsGetWithResponse(ctx, a)
	if e != nil {
		return nil, e
	}
	if r.JSON200 == nil {
		return nil, apiError(r.StatusCode(), r.Body)
	}
	return *r.JSON200, nil
}
func (s generatedService) CashflowCreate(ctx context.Context, a string, v daikuv1.AssetCashFlowRequest) (*daikuv1.AssetCashFlow, error) {
	r, e := s.c.DaikuAssetsIdCashflowsPostWithResponse(ctx, a, v)
	if e != nil {
		return nil, e
	}
	if r.JSON201 == nil {
		return nil, apiError(r.StatusCode(), r.Body)
	}
	return r.JSON201, nil
}
func (s generatedService) CashflowUpdate(ctx context.Context, a, id string, v daikuv1.PatchedAssetCashFlowRequest) (*daikuv1.AssetCashFlow, error) {
	r, e := s.c.DaikuAssetsIdCashflowsCfPkPatchWithResponse(ctx, a, id, v)
	if e != nil {
		return nil, e
	}
	if r.JSON200 == nil {
		return nil, apiError(r.StatusCode(), r.Body)
	}
	return r.JSON200, nil
}
func (s generatedService) CashflowDelete(ctx context.Context, a, id string) error {
	r, e := s.c.DaikuAssetsIdCashflowsCfPkDeleteWithResponse(ctx, a, id)
	if e != nil {
		return e
	}
	if r.StatusCode() != 204 {
		return apiError(r.StatusCode(), r.Body)
	}
	return nil
}
func (s generatedService) HistoryList(ctx context.Context, a string) ([]daikuv1.AssetValueHistory, error) {
	r, e := s.c.DaikuAssetsIdValueHistoryGetWithResponse(ctx, a)
	if e != nil {
		return nil, e
	}
	if r.JSON200 == nil {
		return nil, apiError(r.StatusCode(), r.Body)
	}
	return *r.JSON200, nil
}
func (s generatedService) HistoryCreate(ctx context.Context, a string, v daikuv1.AssetValueHistoryRequest) (*daikuv1.AssetValueHistory, error) {
	r, e := s.c.DaikuAssetsIdValueHistoryPostWithResponse(ctx, a, v)
	if e != nil {
		return nil, e
	}
	if r.JSON201 != nil {
		return r.JSON201, nil
	}
	if r.JSON200 != nil {
		return r.JSON200, nil
	}
	return nil, apiError(r.StatusCode(), r.Body)
}
func (s generatedService) HistoryUpdate(ctx context.Context, a, id string, v daikuv1.PatchedAssetValueHistoryRequest) (*daikuv1.AssetValueHistory, error) {
	r, e := s.c.DaikuAssetsIdValueHistoryVhPkPatchWithResponse(ctx, a, id, v)
	if e != nil {
		return nil, e
	}
	if r.JSON200 == nil {
		return nil, apiError(r.StatusCode(), r.Body)
	}
	return r.JSON200, nil
}
func (s generatedService) HistoryDelete(ctx context.Context, a, id string) error {
	r, e := s.c.DaikuAssetsIdValueHistoryVhPkDeleteWithResponse(ctx, a, id)
	if e != nil {
		return e
	}
	if r.StatusCode() != 204 {
		return apiError(r.StatusCode(), r.Body)
	}
	return nil
}
