// Package apps implements discovery tools: listing apps and reading an app's
// appInfo together with its current appInfo localizations.
package apps

//go:generate go tool mockgen -source=service.go -destination=mocks/mock_ascclient.go -package=mocks ascClient

import (
	"context"

	"github.com/tergrigoryantc/asc-mcp/internal/shared/asc"
	"github.com/tergrigoryantc/asc-mcp/internal/shared/store"
	"github.com/tergrigoryantc/asc-mcp/internal/shared/toolkit"
)

// ascClient is the subset of the ASC facade this feature needs. Declaring it on
// the consumer side keeps the service testable with a generated mock.
type ascClient interface {
	ListApps(ctx context.Context) ([]asc.Resource, error)
	GetApp(ctx context.Context, id string) (asc.Resource, error)
	GetAppInfos(ctx context.Context, appID string) ([]asc.Resource, error)
	GetAppInfoLocalizations(ctx context.Context, appInfoID string) ([]asc.Resource, error)

	// App-level availability & price.
	GetAllTerritories(ctx context.Context) ([]asc.Resource, error)
	GetAppPricePoints(ctx context.Context, appID, territory string) ([]asc.Resource, error)
	CreateAppAvailability(ctx context.Context, payload any) (asc.Resource, error)
	CreateAppPriceSchedule(ctx context.Context, payload any) (asc.Resource, error)
}

// Service holds app discovery and app-level availability/price logic.
type Service struct {
	asc    ascClient
	store  store.Store
	dryRun bool
}

// NewService builds the apps service from shared deps.
func NewService(d *toolkit.Deps) *Service {
	return &Service{asc: d.ASC, store: d.Store, dryRun: d.DryRun}
}

// AppSummary is the trimmed representation returned by list_apps.
type AppSummary struct {
	ID       string `json:"id"`
	BundleID string `json:"bundleId"`
	Name     string `json:"name"`
	SKU      string `json:"sku,omitempty"`
}

type appAttributes struct {
	BundleID string `json:"bundleId"`
	Name     string `json:"name"`
	SKU      string `json:"sku"`
}

// ListApps returns every app visible to the credentials.
func (s *Service) ListApps(ctx context.Context) ([]AppSummary, error) {
	resources, err := s.asc.ListApps(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]AppSummary, 0, len(resources))
	for _, r := range resources {
		var a appAttributes
		_ = r.Attr(&a)
		out = append(out, AppSummary{ID: r.ID, BundleID: a.BundleID, Name: a.Name, SKU: a.SKU})
	}
	return out, nil
}

// AppInfoLocalization is one localized appInfo record.
type AppInfoLocalization struct {
	ID               string `json:"id"`
	Locale           string `json:"locale"`
	Name             string `json:"name,omitempty"`
	Subtitle         string `json:"subtitle,omitempty"`
	PrivacyPolicyURL string `json:"privacyPolicyUrl,omitempty"`
}

type appInfoLocAttributes struct {
	Locale           string `json:"locale"`
	Name             string `json:"name"`
	Subtitle         string `json:"subtitle"`
	PrivacyPolicyURL string `json:"privacyPolicyUrl"`
}

// AppInfo couples an appInfo resource with its localizations.
type AppInfo struct {
	ID            string                `json:"id"`
	Localizations []AppInfoLocalization `json:"localizations"`
}

// AppInfoResult is the full get_app_info payload.
type AppInfoResult struct {
	App      AppSummary `json:"app"`
	AppInfos []AppInfo  `json:"appInfos"`
}

// GetAppInfo returns the app summary plus each appInfo's localizations.
func (s *Service) GetAppInfo(ctx context.Context, appID string) (*AppInfoResult, error) {
	appRes, err := s.asc.GetApp(ctx, appID)
	if err != nil {
		return nil, err
	}
	var a appAttributes
	_ = appRes.Attr(&a)

	infos, err := s.asc.GetAppInfos(ctx, appID)
	if err != nil {
		return nil, err
	}

	result := &AppInfoResult{
		App:      AppSummary{ID: appRes.ID, BundleID: a.BundleID, Name: a.Name, SKU: a.SKU},
		AppInfos: make([]AppInfo, 0, len(infos)),
	}
	for _, info := range infos {
		locs, err := s.asc.GetAppInfoLocalizations(ctx, info.ID)
		if err != nil {
			return nil, err
		}
		ai := AppInfo{ID: info.ID, Localizations: make([]AppInfoLocalization, 0, len(locs))}
		for _, l := range locs {
			var la appInfoLocAttributes
			_ = l.Attr(&la)
			ai.Localizations = append(ai.Localizations, AppInfoLocalization{
				ID: l.ID, Locale: la.Locale, Name: la.Name,
				Subtitle: la.Subtitle, PrivacyPolicyURL: la.PrivacyPolicyURL,
			})
		}
		result.AppInfos = append(result.AppInfos, ai)
	}
	return result, nil
}
