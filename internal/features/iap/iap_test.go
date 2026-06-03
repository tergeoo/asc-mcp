package iap

import (
	"context"
	"encoding/json"
	"testing"

	"go.uber.org/mock/gomock"

	"github.com/tergeoo/asc-mcp/internal/features/iap/mocks"
	"github.com/tergeoo/asc-mcp/internal/shared/asc"
	"github.com/tergeoo/asc-mcp/internal/shared/store"
)

func r(id string, attrs map[string]string) asc.Resource {
	b, _ := json.Marshal(attrs)
	return asc.Resource{ID: id, Attributes: b}
}

func newSvc(t *testing.T) (*Service, *mocks.MockascClient) {
	t.Helper()
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)
	m := mocks.NewMockascClient(ctrl)
	return &Service{asc: m, store: store.Noop{}}, m
}

func TestListCombinesIAPsAndSubscriptions(t *testing.T) {
	s, m := newSvc(t)
	ctx := context.Background()
	m.EXPECT().GetAppIAPs(ctx, "app-1").Return([]asc.Resource{
		r("iap-1", map[string]string{"name": "Coins", "productId": "com.a.coins", "inAppPurchaseType": "CONSUMABLE"}),
	}, nil)
	m.EXPECT().GetAppSubscriptionGroups(ctx, "app-1").Return([]asc.Resource{r("grp-1", nil)}, nil)
	m.EXPECT().GetGroupSubscriptions(ctx, "grp-1").Return([]asc.Resource{
		r("sub-1", map[string]string{"name": "Pro", "productId": "com.a.pro"}),
	}, nil)

	out, err := s.List(ctx, "app-1")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(out.IAPs) != 1 || out.IAPs[0].Type != "CONSUMABLE" {
		t.Fatalf("iaps: %+v", out.IAPs)
	}
	if len(out.Subscriptions) != 1 || out.Subscriptions[0].ID != "sub-1" {
		t.Fatalf("subs: %+v", out.Subscriptions)
	}
}

func TestUpdateIAPLocalizationUpdatesExisting(t *testing.T) {
	s, m := newSvc(t)
	ctx := context.Background()
	m.EXPECT().GetIAPLocalizations(ctx, "iap-1").Return([]asc.Resource{
		r("loc-en", map[string]string{"locale": "en-US"}),
	}, nil)
	m.EXPECT().UpdateIAPLocalization(ctx, "loc-en", gomock.Any()).
		Return(r("loc-en", map[string]string{"locale": "en-US", "name": "Coins"}), nil)

	out, err := s.UpdateIAPLocalization(ctx, UpdateIAPLocalizationInput{
		IAPID: "iap-1", Locale: "en-US", Fields: LocFields{Name: "Coins"},
	})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if out.Result.(map[string]any)["id"] != "loc-en" {
		t.Fatalf("unexpected: %+v", out.Result)
	}
}

func TestUpdateIAPLocalizationCreatesMissing(t *testing.T) {
	s, m := newSvc(t)
	ctx := context.Background()
	m.EXPECT().GetIAPLocalizations(ctx, "iap-1").Return(nil, nil)
	m.EXPECT().CreateIAPLocalization(ctx, gomock.Any()).
		Return(r("loc-ru", map[string]string{"locale": "ru", "name": "Монеты"}), nil)

	out, err := s.UpdateIAPLocalization(ctx, UpdateIAPLocalizationInput{
		IAPID: "iap-1", Locale: "ru", Fields: LocFields{Name: "Монеты"},
	})
	if err != nil {
		t.Fatalf("create path: %v", err)
	}
	if out.Result.(map[string]any)["id"] != "loc-ru" {
		t.Fatalf("unexpected: %+v", out.Result)
	}
}

func TestCreateIAPValidates(t *testing.T) {
	s, _ := newSvc(t)
	if _, err := s.Create(context.Background(), CreateInput{AppID: "a", Name: "", ProductID: "p", Type: "CONSUMABLE"}); err == nil {
		t.Fatal("expected validation error for empty name")
	}
}
