package apps

import (
	"context"
	"fmt"
	"math"
	"strconv"

	"github.com/tergrigoryantc/asc-mcp/internal/shared/asc"
	"github.com/tergrigoryantc/asc-mcp/internal/shared/toolkit"
	"github.com/tergrigoryantc/asc-mcp/internal/shared/validate"
)

const defaultTerritory = "USA"

// SetAvailability sets the territories an app is available in. With
// allTerritories=true every App Store territory is enabled (and future ones via
// availableInNewTerritories). App availability uses inline territoryAvailability
// objects, one per territory.
func (s *Service) SetAvailability(ctx context.Context, appID string, allTerritories bool, territories []string) (toolkit.Outcome, error) {
	if e := validate.NewBuilder().Required("appId", appID).Result(); e != nil {
		return toolkit.Outcome{}, e
	}
	return toolkit.RunIdempotent(ctx, s.store, s.dryRun,
		toolkit.IdemKey{Tool: "set_app_availability", AppID: appID, Input: map[string]any{"appId": appID, "all": allTerritories, "territories": territories}},
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
			manual := make([]any, 0, len(ids))
			included := make([]any, 0, len(ids))
			for _, id := range ids {
				// Inline-created entities must use a local id of the form ${...}.
				tmp := "${ta-" + id + "}"
				manual = append(manual, map[string]any{"type": "territoryAvailabilities", "id": tmp})
				included = append(included, map[string]any{
					"type":       "territoryAvailabilities",
					"id":         tmp,
					"attributes": map[string]any{"available": true},
					"relationships": map[string]any{
						"territory": asc.ToOne("territories", id),
					},
				})
			}
			payload := map[string]any{
				"data": map[string]any{
					"type":       "appAvailabilities",
					"attributes": map[string]any{"availableInNewTerritories": allTerritories},
					"relationships": map[string]any{
						"app":                     asc.ToOne("apps", appID),
						"territoryAvailabilities": map[string]any{"data": manual},
					},
				},
				"included": included,
			}
			r, err := s.asc.CreateAppAvailability(ctx, payload)
			if err != nil {
				return nil, err
			}
			return map[string]any{"id": r.ID, "appId": appID, "territories": len(ids)}, nil
		},
	)
}

// SetPrice sets an app's price by resolving the target customer price to an app
// price point in the base territory (App Store Connect equalizes the rest). Use
// price "0" for a free app.
func (s *Service) SetPrice(ctx context.Context, appID, territory, price string) (toolkit.Outcome, error) {
	if territory == "" {
		territory = defaultTerritory
	}
	if e := validate.NewBuilder().Required("appId", appID).Required("price", price).Result(); e != nil {
		return toolkit.Outcome{}, e
	}
	return toolkit.RunIdempotent(ctx, s.store, s.dryRun,
		toolkit.IdemKey{Tool: "set_app_price", AppID: appID, Input: map[string]any{"appId": appID, "territory": territory, "price": price}},
		func() (any, error) {
			points, err := s.asc.GetAppPricePoints(ctx, appID, territory)
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
					"type": "appPriceSchedules",
					"relationships": map[string]any{
						"app":           asc.ToOne("apps", appID),
						"baseTerritory": asc.ToOne("territories", territory),
						"manualPrices":  map[string]any{"data": []any{map[string]any{"type": "appPrices", "id": tmp}}},
					},
				},
				"included": []any{
					map[string]any{
						"type":          "appPrices",
						"id":            tmp,
						"relationships": map[string]any{"appPricePoint": asc.ToOne("appPricePoints", ppID)},
					},
				},
			}
			r, err := s.asc.CreateAppPriceSchedule(ctx, payload)
			if err != nil {
				return nil, err
			}
			free := false
			if v, e := strconv.ParseFloat(actual, 64); e == nil && v == 0 {
				free = true
			}
			return map[string]any{"id": r.ID, "appId": appID, "territory": territory, "customerPrice": actual, "free": free}, nil
		},
	)
}

type pricePointAttrs struct {
	CustomerPrice string `json:"customerPrice"`
}

// pickPricePoint resolves a target customer price to a price-point id (exact
// match preferred, else nearest available).
func pickPricePoint(points []asc.Resource, target string) (id, actual string, err error) {
	if len(points) == 0 {
		return "", "", fmt.Errorf("no price points returned for territory")
	}
	want, perr := strconv.ParseFloat(target, 64)
	if perr != nil {
		return "", "", fmt.Errorf("invalid price %q: %w", target, perr)
	}
	bestID, bestPrice, bestDiff := "", "", math.MaxFloat64
	for _, p := range points {
		var a pricePointAttrs
		_ = p.Attr(&a)
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
