package versions

import (
	"context"
	"encoding/json"
	"testing"

	"go.uber.org/mock/gomock"

	"github.com/tergrigoryantc/asc-mcp/internal/features/versions/mocks"
	"github.com/tergrigoryantc/asc-mcp/internal/shared/asc"
	"github.com/tergrigoryantc/asc-mcp/internal/shared/store"
)

func ver(id, vs, state string) asc.Resource {
	attrs, _ := json.Marshal(map[string]string{"versionString": vs, "appStoreState": state, "platform": "IOS"})
	return asc.Resource{ID: id, Attributes: attrs}
}

func svc(t *testing.T) (*Service, *mocks.MockascClient) {
	t.Helper()
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)
	m := mocks.NewMockascClient(ctrl)
	return &Service{asc: m, store: store.Noop{}}, m
}

func TestListFiltersByState(t *testing.T) {
	s, m := svc(t)
	ctx := context.Background()
	m.EXPECT().GetAppVersions(ctx, "app-1").Return([]asc.Resource{
		ver("v1", "1.0", "READY_FOR_SALE"),
		ver("v2", "1.1", "PREPARE_FOR_SUBMISSION"),
	}, nil)

	out, err := s.List(ctx, "app-1", "PREPARE_FOR_SUBMISSION")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(out) != 1 || out[0].ID != "v2" {
		t.Fatalf("filter failed: %+v", out)
	}
}

func TestCreateVersion(t *testing.T) {
	s, m := svc(t)
	ctx := context.Background()
	m.EXPECT().CreateVersion(ctx, gomock.Any()).
		DoAndReturn(func(_ context.Context, payload any) (asc.Resource, error) {
			data := payload.(map[string]any)["data"].(map[string]any)
			rels := data["relationships"].(map[string]any)
			if _, ok := rels["app"]; !ok {
				t.Fatal("create payload must reference the app relationship")
			}
			return ver("v-new", "2.0", "PREPARE_FOR_SUBMISSION"), nil
		})

	out, err := s.Create(ctx, CreateInput{AppID: "app-1", VersionString: "2.0", Platform: "IOS"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if out.Result.(Version).ID != "v-new" {
		t.Fatalf("unexpected: %+v", out.Result)
	}
}

func TestUpdateVersion(t *testing.T) {
	s, m := svc(t)
	ctx := context.Background()
	m.EXPECT().UpdateVersion(ctx, "v1", gomock.Any()).Return(ver("v1", "1.0", "PREPARE_FOR_SUBMISSION"), nil)
	if _, err := s.Update(ctx, UpdateInput{VersionID: "v1", Copyright: "2026 Acme"}); err != nil {
		t.Fatalf("update: %v", err)
	}
}
