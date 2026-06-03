package submission

import (
	"context"
	"encoding/json"
	"testing"

	"go.uber.org/mock/gomock"

	"github.com/tergrigoryantc/asc-mcp/internal/features/submission/mocks"
	"github.com/tergrigoryantc/asc-mcp/internal/shared/asc"
	"github.com/tergrigoryantc/asc-mcp/internal/shared/store"
)

func sub(id, state string) asc.Resource {
	attrs, _ := json.Marshal(map[string]string{"platform": "IOS", "state": state})
	return asc.Resource{ID: id, Attributes: attrs}
}

func newSvc(t *testing.T) (*Service, *mocks.MockascClient) {
	t.Helper()
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)
	m := mocks.NewMockascClient(ctrl)
	return &Service{asc: m, store: store.Noop{}}, m
}

func TestCreateSubmission(t *testing.T) {
	svc, m := newSvc(t)
	ctx := context.Background()
	m.EXPECT().CreateReviewSubmission(ctx, gomock.Any()).Return(sub("sub-1", "READY_FOR_REVIEW"), nil)

	out, err := svc.Create(ctx, "app-1", "IOS")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if out.Result.(map[string]any)["id"] != "sub-1" {
		t.Fatalf("unexpected: %+v", out.Result)
	}
}

func TestCreateSubmissionValidates(t *testing.T) {
	svc, _ := newSvc(t)
	if _, err := svc.Create(context.Background(), "", "IOS"); err == nil {
		t.Fatal("expected validation error for empty appId")
	}
}

func TestAddVersionAndSubmit(t *testing.T) {
	svc, m := newSvc(t)
	ctx := context.Background()

	m.EXPECT().CreateReviewSubmissionItem(ctx, gomock.Any()).Return(asc.Resource{ID: "item-1"}, nil)
	if _, err := svc.AddVersion(ctx, "sub-1", "ver-1"); err != nil {
		t.Fatalf("add version: %v", err)
	}

	m.EXPECT().UpdateReviewSubmission(ctx, "sub-1", gomock.Any()).
		DoAndReturn(func(_ context.Context, _ string, payload any) (asc.Resource, error) {
			attrs := payload.(map[string]any)["data"].(map[string]any)["attributes"].(map[string]any)
			if attrs["submitted"] != true {
				t.Fatal("submit must set submitted=true")
			}
			return sub("sub-1", "WAITING_FOR_REVIEW"), nil
		})
	out, err := svc.Submit(ctx, "sub-1")
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if out.Result.(map[string]any)["state"] != "WAITING_FOR_REVIEW" {
		t.Fatalf("unexpected state: %+v", out.Result)
	}
}

func TestStatus(t *testing.T) {
	svc, m := newSvc(t)
	ctx := context.Background()
	m.EXPECT().GetReviewSubmission(ctx, "sub-1").Return(sub("sub-1", "IN_REVIEW"), nil)
	out, err := svc.Status(ctx, "sub-1")
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if out["state"] != "IN_REVIEW" {
		t.Fatalf("state = %v", out["state"])
	}
}
