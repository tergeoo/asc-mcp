package apps

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"go.uber.org/mock/gomock"

	"github.com/tergrigoryantc/asc-mcp/internal/features/apps/mocks"
	"github.com/tergrigoryantc/asc-mcp/internal/shared/asc"
)

func res(id string, attrs map[string]string) asc.Resource {
	b, _ := json.Marshal(attrs)
	return asc.Resource{ID: id, Attributes: b}
}

func TestListApps(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	m := mocks.NewMockascClient(ctrl)
	svc := &Service{asc: m}
	ctx := context.Background()

	m.EXPECT().ListApps(ctx).Return([]asc.Resource{
		res("1", map[string]string{"bundleId": "com.a", "name": "A", "sku": "SKU-A"}),
		res("2", map[string]string{"bundleId": "com.b", "name": "B"}),
	}, nil)

	apps, err := svc.ListApps(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(apps) != 2 {
		t.Fatalf("expected 2 apps, got %d", len(apps))
	}
	if apps[0].BundleID != "com.a" || apps[0].SKU != "SKU-A" {
		t.Fatalf("unexpected first app: %+v", apps[0])
	}
}

func TestGetAppInfo(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	m := mocks.NewMockascClient(ctrl)
	svc := &Service{asc: m}
	ctx := context.Background()

	m.EXPECT().GetApp(ctx, "app-1").Return(res("app-1", map[string]string{"bundleId": "com.a", "name": "A"}), nil)
	m.EXPECT().GetAppInfos(ctx, "app-1").Return([]asc.Resource{res("info-1", nil)}, nil)
	m.EXPECT().GetAppInfoLocalizations(ctx, "info-1").Return([]asc.Resource{
		res("loc-1", map[string]string{"locale": "en-US", "name": "A", "subtitle": "sub"}),
	}, nil)

	out, err := svc.GetAppInfo(ctx, "app-1")
	if err != nil {
		t.Fatalf("get_app_info: %v", err)
	}
	if out.App.BundleID != "com.a" {
		t.Fatalf("app bundleId = %s", out.App.BundleID)
	}
	if len(out.AppInfos) != 1 || len(out.AppInfos[0].Localizations) != 1 {
		t.Fatalf("unexpected appInfos: %+v", out.AppInfos)
	}
	if out.AppInfos[0].Localizations[0].Locale != "en-US" {
		t.Fatalf("locale = %s", out.AppInfos[0].Localizations[0].Locale)
	}
}

func TestGetAppInfoPropagatesError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	m := mocks.NewMockascClient(ctrl)
	svc := &Service{asc: m}
	ctx := context.Background()

	boom := errors.New("boom")
	m.EXPECT().GetApp(ctx, "x").Return(asc.Resource{}, boom)

	if _, err := svc.GetAppInfo(ctx, "x"); !errors.Is(err, boom) {
		t.Fatalf("expected propagated error, got %v", err)
	}
}
