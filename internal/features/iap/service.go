// Package iap implements in-app purchase and subscription tools: listing,
// creating IAPs and updating IAP/subscription localizations.
package iap

//go:generate go tool mockgen -source=service.go -destination=mocks/mock_ascclient.go -package=mocks ascClient

import (
	"context"

	"github.com/tergrigoryantc/asc-mcp/internal/shared/asc"
	"github.com/tergrigoryantc/asc-mcp/internal/shared/store"
	"github.com/tergrigoryantc/asc-mcp/internal/shared/toolkit"
	"github.com/tergrigoryantc/asc-mcp/internal/shared/validate"
)

type ascClient interface {
	GetAppIAPs(ctx context.Context, appID string) ([]asc.Resource, error)
	CreateIAP(ctx context.Context, payload any) (asc.Resource, error)
	GetIAPLocalizations(ctx context.Context, iapID string) ([]asc.Resource, error)
	CreateIAPLocalization(ctx context.Context, payload any) (asc.Resource, error)
	UpdateIAPLocalization(ctx context.Context, id string, payload any) (asc.Resource, error)
	GetAppSubscriptionGroups(ctx context.Context, appID string) ([]asc.Resource, error)
	GetGroupSubscriptions(ctx context.Context, groupID string) ([]asc.Resource, error)
	GetSubscriptionLocalizations(ctx context.Context, subscriptionID string) ([]asc.Resource, error)
	CreateSubscriptionLocalization(ctx context.Context, payload any) (asc.Resource, error)
	UpdateSubscriptionLocalization(ctx context.Context, id string, payload any) (asc.Resource, error)

	// Product & price creation.
	CreateSubscriptionGroup(ctx context.Context, payload any) (asc.Resource, error)
	CreateSubscriptionGroupLocalization(ctx context.Context, payload any) (asc.Resource, error)
	CreateSubscription(ctx context.Context, payload any) (asc.Resource, error)
	GetAllTerritories(ctx context.Context) ([]asc.Resource, error)
	CreateSubscriptionAvailability(ctx context.Context, payload any) (asc.Resource, error)
	CreateIAPAvailability(ctx context.Context, payload any) (asc.Resource, error)
	CreateSubscriptionPrice(ctx context.Context, payload any) (asc.Resource, error)
	GetSubscriptionPricePoints(ctx context.Context, subID, territory string) ([]asc.Resource, error)
	GetSubscriptionPricePointEqualizations(ctx context.Context, pointID string) ([]asc.Resource, error)
	CreateIAPPriceSchedule(ctx context.Context, payload any) (asc.Resource, error)
	GetIAPPricePoints(ctx context.Context, iapID, territory string) ([]asc.Resource, error)
}

// Service holds IAP logic.
type Service struct {
	asc    ascClient
	store  store.Store
	dryRun bool
}

// NewService builds the IAP service.
func NewService(d *toolkit.Deps) *Service {
	return &Service{asc: d.ASC, store: d.Store, dryRun: d.DryRun}
}

// IAP is the trimmed inAppPurchase representation.
type IAP struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	ProductID string `json:"productId"`
	Type      string `json:"type,omitempty"`
	State     string `json:"state,omitempty"`
}

type iapAttrs struct {
	Name             string `json:"name"`
	ProductID        string `json:"productId"`
	InAppPurchaseType string `json:"inAppPurchaseType"`
	State            string `json:"state"`
}

// Subscription is the trimmed subscription representation.
type Subscription struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	ProductID string `json:"productId"`
	State     string `json:"state,omitempty"`
}

type subAttrs struct {
	Name      string `json:"name"`
	ProductID string `json:"productId"`
	State     string `json:"state"`
}

// ListResult holds IAPs and subscriptions for an app.
type ListResult struct {
	IAPs          []IAP          `json:"iaps"`
	Subscriptions []Subscription `json:"subscriptions"`
}

// List returns the app's IAPs (v2) and subscriptions.
func (s *Service) List(ctx context.Context, appID string) (*ListResult, error) {
	result := &ListResult{IAPs: []IAP{}, Subscriptions: []Subscription{}}

	iaps, err := s.asc.GetAppIAPs(ctx, appID)
	if err != nil {
		return nil, err
	}
	for _, r := range iaps {
		var a iapAttrs
		_ = r.Attr(&a)
		result.IAPs = append(result.IAPs, IAP{
			ID: r.ID, Name: a.Name, ProductID: a.ProductID, Type: a.InAppPurchaseType, State: a.State,
		})
	}

	groups, err := s.asc.GetAppSubscriptionGroups(ctx, appID)
	if err != nil {
		return nil, err
	}
	for _, g := range groups {
		subs, err := s.asc.GetGroupSubscriptions(ctx, g.ID)
		if err != nil {
			return nil, err
		}
		for _, r := range subs {
			var a subAttrs
			_ = r.Attr(&a)
			result.Subscriptions = append(result.Subscriptions, Subscription{
				ID: r.ID, Name: a.Name, ProductID: a.ProductID, State: a.State,
			})
		}
	}
	return result, nil
}

// CreateInput holds create_iap parameters.
type CreateInput struct {
	AppID     string `json:"appId"`
	Name      string `json:"name"`
	ProductID string `json:"productId"`
	Type      string `json:"type"`
	ReviewNote string `json:"reviewNote,omitempty"`
}

// Create makes a consumable/non-consumable IAP (v2).
func (s *Service) Create(ctx context.Context, in CreateInput) (toolkit.Outcome, error) {
	b := validate.NewBuilder()
	b.Required("name", in.Name).Required("productId", in.ProductID).Required("type", in.Type)
	b.MaxLen("name", in.Name, validate.MaxIAPName)
	if e := b.Result(); e != nil {
		return toolkit.Outcome{}, e
	}
	attrs := map[string]any{
		"name":              in.Name,
		"productId":         in.ProductID,
		"inAppPurchaseType": in.Type,
		"reviewNote":        in.ReviewNote,
	}
	payload := asc.Create("inAppPurchases", attrs, map[string]any{"app": asc.ToOne("apps", in.AppID)})
	return toolkit.RunIdempotent(ctx, s.store, s.dryRun,
		toolkit.IdemKey{Tool: "create_iap", AppID: in.AppID, Input: in},
		func() (any, error) {
			r, err := s.asc.CreateIAP(ctx, payload)
			if err != nil {
				return nil, err
			}
			var a iapAttrs
			_ = r.Attr(&a)
			return IAP{ID: r.ID, Name: a.Name, ProductID: a.ProductID, Type: a.InAppPurchaseType, State: a.State}, nil
		},
	)
}

// LocFields are the localizable name/description.
type LocFields struct {
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
}

// UpdateIAPLocalizationInput updates (or creates) an IAP localization by locale.
type UpdateIAPLocalizationInput struct {
	IAPID  string    `json:"iapId"`
	Locale string    `json:"locale"`
	Fields LocFields `json:"fields"`
}

// UpdateIAPLocalization upserts an IAP localization for a locale.
func (s *Service) UpdateIAPLocalization(ctx context.Context, in UpdateIAPLocalizationInput) (toolkit.Outcome, error) {
	b := validate.NewBuilder().Required("locale", in.Locale)
	b.MaxLen("name", in.Fields.Name, validate.MaxIAPName)
	b.MaxLen("description", in.Fields.Description, validate.MaxIAPDescription)
	if e := b.Result(); e != nil {
		return toolkit.Outcome{}, e
	}
	return toolkit.RunIdempotent(ctx, s.store, s.dryRun,
		toolkit.IdemKey{Tool: "update_iap_localization", Input: in},
		func() (any, error) {
			existing, err := s.asc.GetIAPLocalizations(ctx, in.IAPID)
			if err != nil {
				return nil, err
			}
			if id := findLocale(existing, in.Locale); id != "" {
				payload := asc.Update("inAppPurchaseLocalizations", id, map[string]any{
					"name": in.Fields.Name, "description": in.Fields.Description,
				})
				r, err := s.asc.UpdateIAPLocalization(ctx, id, payload)
				if err != nil {
					return nil, err
				}
				return locResult(r), nil
			}
			payload := asc.Create("inAppPurchaseLocalizations", map[string]any{
				"locale": in.Locale, "name": in.Fields.Name, "description": in.Fields.Description,
			}, map[string]any{"inAppPurchaseV2": asc.ToOne("inAppPurchases", in.IAPID)})
			r, err := s.asc.CreateIAPLocalization(ctx, payload)
			if err != nil {
				return nil, err
			}
			return locResult(r), nil
		},
	)
}

// UpdateSubscriptionLocalizationInput upserts a subscription localization.
type UpdateSubscriptionLocalizationInput struct {
	SubscriptionID string    `json:"subscriptionId"`
	Locale         string    `json:"locale"`
	Fields         LocFields `json:"fields"`
}

// UpdateSubscriptionLocalization upserts a subscription localization for a locale.
func (s *Service) UpdateSubscriptionLocalization(ctx context.Context, in UpdateSubscriptionLocalizationInput) (toolkit.Outcome, error) {
	if e := validate.NewBuilder().Required("locale", in.Locale).Result(); e != nil {
		return toolkit.Outcome{}, e
	}
	return toolkit.RunIdempotent(ctx, s.store, s.dryRun,
		toolkit.IdemKey{Tool: "update_subscription_localization", Input: in},
		func() (any, error) {
			existing, err := s.asc.GetSubscriptionLocalizations(ctx, in.SubscriptionID)
			if err != nil {
				return nil, err
			}
			if id := findLocale(existing, in.Locale); id != "" {
				payload := asc.Update("subscriptionLocalizations", id, map[string]any{
					"name": in.Fields.Name, "description": in.Fields.Description,
				})
				r, err := s.asc.UpdateSubscriptionLocalization(ctx, id, payload)
				if err != nil {
					return nil, err
				}
				return locResult(r), nil
			}
			payload := asc.Create("subscriptionLocalizations", map[string]any{
				"locale": in.Locale, "name": in.Fields.Name, "description": in.Fields.Description,
			}, map[string]any{"subscription": asc.ToOne("subscriptions", in.SubscriptionID)})
			r, err := s.asc.CreateSubscriptionLocalization(ctx, payload)
			if err != nil {
				return nil, err
			}
			return locResult(r), nil
		},
	)
}

type localeOnly struct {
	Locale string `json:"locale"`
}

// findLocale returns the resource id matching a locale, or "".
func findLocale(resources []asc.Resource, locale string) string {
	for _, r := range resources {
		var a localeOnly
		_ = r.Attr(&a)
		if a.Locale == locale {
			return r.ID
		}
	}
	return ""
}

func locResult(r asc.Resource) map[string]any {
	var a struct {
		Locale      string `json:"locale"`
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	_ = r.Attr(&a)
	return map[string]any{
		"id": r.ID, "locale": a.Locale, "name": a.Name, "description": a.Description,
	}
}
