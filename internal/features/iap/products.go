package iap

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/tergeoo/asc-mcp/internal/shared/asc"
	"github.com/tergeoo/asc-mcp/internal/shared/mcperr"
	"github.com/tergeoo/asc-mcp/internal/shared/toolkit"
	"github.com/tergeoo/asc-mcp/internal/shared/validate"
)

// defaultTerritory is the base territory used to set a price; App Store Connect
// then automatically equalizes prices across other territories.
const defaultTerritory = "USA"

// --- Subscription groups ---------------------------------------------------

// CreateSubscriptionGroup creates a subscription group for an app.
func (s *Service) CreateSubscriptionGroup(ctx context.Context, appID, referenceName string) (toolkit.Outcome, error) {
	if e := validate.NewBuilder().Required("appId", appID).Required("referenceName", referenceName).Result(); e != nil {
		return toolkit.Outcome{}, e
	}
	payload := asc.Create("subscriptionGroups",
		map[string]any{"referenceName": referenceName},
		map[string]any{"app": asc.ToOne("apps", appID)},
	)
	return toolkit.RunIdempotent(ctx, s.store, s.dryRun,
		toolkit.IdemKey{Tool: "create_subscription_group", AppID: appID, Input: map[string]any{"appId": appID, "referenceName": referenceName}},
		func() (any, error) {
			r, err := s.asc.CreateSubscriptionGroup(ctx, payload)
			if err != nil {
				return nil, err
			}
			// Group display name (per-locale) is required metadata; set it for the locale.
			return map[string]any{"id": r.ID, "referenceName": referenceName}, nil
		},
	)
}

// SetGroupLocalization sets a subscription group's localized display name.
func (s *Service) SetGroupLocalization(ctx context.Context, groupID, locale, name string) (toolkit.Outcome, error) {
	b := validate.NewBuilder().Required("groupId", groupID).Required("locale", locale).Required("name", name)
	if e := b.Result(); e != nil {
		return toolkit.Outcome{}, e
	}
	payload := asc.Create("subscriptionGroupLocalizations",
		map[string]any{"name": name, "locale": locale},
		map[string]any{"subscriptionGroup": asc.ToOne("subscriptionGroups", groupID)},
	)
	return toolkit.RunIdempotent(ctx, s.store, s.dryRun,
		toolkit.IdemKey{Tool: "set_subscription_group_localization", Input: map[string]any{"groupId": groupID, "locale": locale, "name": name}},
		func() (any, error) {
			r, err := s.asc.CreateSubscriptionGroupLocalization(ctx, payload)
			if err != nil {
				return nil, err
			}
			return map[string]any{"id": r.ID, "locale": locale, "name": name}, nil
		},
	)
}

// SetSubscriptionAvailability sets the territories a subscription is sold in.
// This is a prerequisite for pricing: a subscription with no availability has
// no valid price points. With allTerritories=true every App Store territory is
// enabled (and future ones via availableInNewTerritories).
func (s *Service) SetSubscriptionAvailability(ctx context.Context, subID string, allTerritories bool, territories []string) (toolkit.Outcome, error) {
	if e := validate.NewBuilder().Required("subscriptionId", subID).Result(); e != nil {
		return toolkit.Outcome{}, e
	}
	return toolkit.RunIdempotent(ctx, s.store, s.dryRun,
		toolkit.IdemKey{Tool: "set_subscription_availability", Input: map[string]any{"subscriptionId": subID, "all": allTerritories, "territories": territories}},
		func() (any, error) {
			ids := territories
			if allTerritories {
				all, err := s.asc.GetAllTerritories(ctx)
				if err != nil {
					return nil, err
				}
				ids = make([]string, 0, len(all))
				for _, t := range all {
					ids = append(ids, t.ID)
				}
			}
			if len(ids) == 0 {
				return nil, fmt.Errorf("no territories specified")
			}
			terrData := make([]any, 0, len(ids))
			for _, id := range ids {
				terrData = append(terrData, map[string]any{"type": "territories", "id": id})
			}
			payload := map[string]any{"data": map[string]any{
				"type":       "subscriptionAvailabilities",
				"attributes": map[string]any{"availableInNewTerritories": allTerritories},
				"relationships": map[string]any{
					"subscription":         asc.ToOne("subscriptions", subID),
					"availableTerritories": map[string]any{"data": terrData},
				},
			}}
			r, err := s.asc.CreateSubscriptionAvailability(ctx, payload)
			if err != nil {
				return nil, err
			}
			return map[string]any{"id": r.ID, "subscriptionId": subID, "territories": len(ids)}, nil
		},
	)
}

// SetIAPAvailability sets the territories a one-time IAP is sold in. Required to
// clear MISSING_METADATA, mirroring subscription availability.
func (s *Service) SetIAPAvailability(ctx context.Context, iapID string, allTerritories bool, territories []string) (toolkit.Outcome, error) {
	if e := validate.NewBuilder().Required("iapId", iapID).Result(); e != nil {
		return toolkit.Outcome{}, e
	}
	return toolkit.RunIdempotent(ctx, s.store, s.dryRun,
		toolkit.IdemKey{Tool: "set_iap_availability", Input: map[string]any{"iapId": iapID, "all": allTerritories, "territories": territories}},
		func() (any, error) {
			ids := territories
			if allTerritories {
				all, err := s.asc.GetAllTerritories(ctx)
				if err != nil {
					return nil, err
				}
				ids = make([]string, 0, len(all))
				for _, t := range all {
					ids = append(ids, t.ID)
				}
			}
			if len(ids) == 0 {
				return nil, fmt.Errorf("no territories specified")
			}
			terrData := make([]any, 0, len(ids))
			for _, id := range ids {
				terrData = append(terrData, map[string]any{"type": "territories", "id": id})
			}
			payload := map[string]any{"data": map[string]any{
				"type":       "inAppPurchaseAvailabilities",
				"attributes": map[string]any{"availableInNewTerritories": allTerritories},
				"relationships": map[string]any{
					"inAppPurchase":        asc.ToOne("inAppPurchases", iapID),
					"availableTerritories": map[string]any{"data": terrData},
				},
			}}
			r, err := s.asc.CreateIAPAvailability(ctx, payload)
			if err != nil {
				return nil, err
			}
			return map[string]any{"id": r.ID, "iapId": iapID, "territories": len(ids)}, nil
		},
	)
}

// --- Subscriptions ---------------------------------------------------------

// CreateSubscriptionInput holds create_subscription parameters.
type CreateSubscriptionInput struct {
	GroupID    string `json:"groupId"`
	Name       string `json:"name"`
	ProductID  string `json:"productId"`
	Period     string `json:"subscriptionPeriod"`
	GroupLevel int    `json:"groupLevel"`
}

// CreateSubscription creates an auto-renewable subscription in a group.
func (s *Service) CreateSubscription(ctx context.Context, in CreateSubscriptionInput) (toolkit.Outcome, error) {
	b := validate.NewBuilder().
		Required("groupId", in.GroupID).
		Required("name", in.Name).
		Required("productId", in.ProductID).
		Required("subscriptionPeriod", in.Period)
	b.MaxLen("name", in.Name, validate.MaxIAPName)
	if e := b.Result(); e != nil {
		return toolkit.Outcome{}, e
	}
	level := in.GroupLevel
	if level <= 0 {
		level = 1
	}
	payload := asc.Create("subscriptions",
		map[string]any{
			"name":               in.Name,
			"productId":          in.ProductID,
			"subscriptionPeriod": in.Period,
			"groupLevel":         level,
		},
		map[string]any{"group": asc.ToOne("subscriptionGroups", in.GroupID)},
	)
	return toolkit.RunIdempotent(ctx, s.store, s.dryRun,
		toolkit.IdemKey{Tool: "create_subscription", Input: in},
		func() (any, error) {
			r, err := s.asc.CreateSubscription(ctx, payload)
			if err != nil {
				return nil, err
			}
			var a subAttrs
			_ = r.Attr(&a)
			return Subscription{ID: r.ID, Name: a.Name, ProductID: a.ProductID, State: a.State}, nil
		},
	)
}

// SetSubscriptionPrice sets the base-territory price for a subscription by
// resolving the target customer price to a price-point id.
func (s *Service) SetSubscriptionPrice(ctx context.Context, subID, territory, price string) (toolkit.Outcome, error) {
	if territory == "" {
		territory = defaultTerritory
	}
	if e := validate.NewBuilder().Required("subscriptionId", subID).Required("price", price).Result(); e != nil {
		return toolkit.Outcome{}, e
	}
	return toolkit.RunIdempotent(ctx, s.store, s.dryRun,
		toolkit.IdemKey{Tool: "set_subscription_price", Input: map[string]any{"subscriptionId": subID, "territory": territory, "price": price}},
		func() (any, error) {
			points, err := s.asc.GetSubscriptionPricePoints(ctx, subID, territory)
			if err != nil {
				return nil, err
			}
			baseID, actual, err := pickPricePoint(points, price)
			if err != nil {
				return nil, err
			}
			// Subscription prices do NOT auto-equalize across territories (unlike
			// app/IAP price schedules). Price the base territory, then price every
			// other territory at the equivalent tier via the point's equalizations,
			// otherwise the subscription stays MISSING_METADATA (priced in only one
			// territory while available in many).
			pointIDs := []string{baseID}
			eq, err := s.asc.GetSubscriptionPricePointEqualizations(ctx, baseID)
			if err != nil {
				return nil, err
			}
			for _, p := range eq {
				pointIDs = append(pointIDs, p.ID)
			}

			created, skipped := 0, 0
			for _, ppID := range pointIDs {
				payload := asc.Create("subscriptionPrices", nil, map[string]any{
					"subscription":           asc.ToOne("subscriptions", subID),
					"subscriptionPricePoint": asc.ToOne("subscriptionPricePoints", ppID),
				})
				if _, err := s.asc.CreateSubscriptionPrice(ctx, payload); err != nil {
					if isAlreadyExists(err) {
						skipped++
						continue
					}
					return nil, fmt.Errorf("price point %s: %w", ppID, err)
				}
				created++
			}
			return map[string]any{
				"subscriptionId": subID, "baseTerritory": territory, "customerPrice": actual,
				"territoriesPriced": created, "alreadyPriced": skipped,
			}, nil
		},
	)
}

// isAlreadyExists reports whether an ASC error indicates the resource already
// exists (so a re-run can skip it idempotently).
func isAlreadyExists(err error) bool {
	var e *mcperr.Error
	if errors.As(err, &e) {
		for _, d := range e.Details {
			if strings.Contains(strings.ToLower(d.Detail), "already exist") {
				return true
			}
		}
		return strings.Contains(strings.ToLower(e.Message), "already exist")
	}
	return false
}

// --- IAP (non-consumable / consumable) price -------------------------------

// SetIAPPrice sets a one-time IAP's price via a price schedule.
func (s *Service) SetIAPPrice(ctx context.Context, iapID, territory, price string) (toolkit.Outcome, error) {
	if territory == "" {
		territory = defaultTerritory
	}
	if e := validate.NewBuilder().Required("iapId", iapID).Required("price", price).Result(); e != nil {
		return toolkit.Outcome{}, e
	}
	return toolkit.RunIdempotent(ctx, s.store, s.dryRun,
		toolkit.IdemKey{Tool: "set_iap_price", Input: map[string]any{"iapId": iapID, "territory": territory, "price": price}},
		func() (any, error) {
			points, err := s.asc.GetIAPPricePoints(ctx, iapID, territory)
			if err != nil {
				return nil, err
			}
			ppID, actual, err := pickPricePoint(points, price)
			if err != nil {
				return nil, err
			}
			const tmp = "${price1}"
			payload := map[string]any{
				"data": map[string]any{
					"type": "inAppPurchasePriceSchedules",
					"relationships": map[string]any{
						"inAppPurchase": asc.ToOne("inAppPurchases", iapID),
						"baseTerritory": asc.ToOne("territories", territory),
						"manualPrices":  map[string]any{"data": []any{map[string]any{"type": "inAppPurchasePrices", "id": tmp}}},
					},
				},
				"included": []any{
					map[string]any{
						"type": "inAppPurchasePrices",
						"id":   tmp,
						"relationships": map[string]any{
							"inAppPurchasePricePoint": asc.ToOne("inAppPurchasePricePoints", ppID),
						},
					},
				},
			}
			r, err := s.asc.CreateIAPPriceSchedule(ctx, payload)
			if err != nil {
				return nil, err
			}
			return map[string]any{"id": r.ID, "iapId": iapID, "territory": territory, "customerPrice": actual, "pricePointId": ppID}, nil
		},
	)
}

type pricePointAttrs struct {
	CustomerPrice string `json:"customerPrice"`
}

// pickPricePoint resolves a target customer price to a price-point id, choosing
// an exact match if present, otherwise the nearest available price.
func pickPricePoint(points []asc.Resource, target string) (id, actual string, err error) {
	if len(points) == 0 {
		return "", "", fmt.Errorf("no price points returned for territory (is the Paid Applications Agreement active?)")
	}
	want, perr := strconv.ParseFloat(target, 64)
	if perr != nil {
		return "", "", fmt.Errorf("invalid price %q: %w", target, perr)
	}
	bestID, bestPrice, bestDiff := "", "", math.MaxFloat64
	for _, p := range points {
		var a pricePointAttrs
		if a.CustomerPrice == "" {
			_ = p.Attr(&a)
		}
		if a.CustomerPrice == target {
			return p.ID, a.CustomerPrice, nil
		}
		if v, e := strconv.ParseFloat(a.CustomerPrice, 64); e == nil {
			if d := math.Abs(v - want); d < bestDiff {
				bestID, bestPrice, bestDiff = p.ID, a.CustomerPrice, d
			}
		}
	}
	if bestID == "" {
		return "", "", fmt.Errorf("no usable price point near %s", target)
	}
	return bestID, bestPrice, nil
}
