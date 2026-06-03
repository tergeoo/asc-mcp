package localization

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"go.uber.org/mock/gomock"

	"github.com/tergrigoryantc/asc-mcp/internal/features/localization/mocks"
	"github.com/tergrigoryantc/asc-mcp/internal/shared/asc"
	"github.com/tergrigoryantc/asc-mcp/internal/shared/store"
	"github.com/tergrigoryantc/asc-mcp/internal/shared/toolkit"
	"github.com/tergrigoryantc/asc-mcp/internal/shared/validate"
)

func locResource(id, locale string) asc.Resource {
	attrs, _ := json.Marshal(map[string]string{"locale": locale})
	return asc.Resource{ID: id, Type: "appStoreVersionLocalizations", Attributes: attrs}
}

func newService(t *testing.T, dryRun bool) (*Service, *mocks.MockascClient) {
	t.Helper()
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)
	m := mocks.NewMockascClient(ctrl)
	return &Service{asc: m, store: store.Noop{}, dryRun: dryRun}, m
}

func TestBulkUpdatesExistingAndCreatesMissing(t *testing.T) {
	svc, m := newService(t, false)
	ctx := context.Background()

	m.EXPECT().GetVersionLocalizations(ctx, "ver-1").
		Return([]asc.Resource{locResource("loc-en", "en-US")}, nil)

	// en-US exists -> update by id.
	m.EXPECT().UpdateVersionLocalization(ctx, "loc-en", gomock.Any()).
		Return(locResource("loc-en", "en-US"), nil)
	// ru is missing -> create.
	m.EXPECT().CreateVersionLocalization(ctx, gomock.Any()).
		Return(locResource("loc-ru", "ru"), nil)

	out, err := svc.BulkUpdateVersionLocalizations(ctx, BulkInput{
		VersionID: "ver-1",
		Localizations: []LocaleFields{
			{Locale: "en-US", Fields: VersionFields{Keywords: "a,b"}},
			{Locale: "ru", Fields: VersionFields{Description: "Описание"}},
		},
	})
	if err != nil {
		t.Fatalf("bulk: %v", err)
	}
	res := out.Result.(BulkResult)
	if len(res.Updated) != 1 || res.Updated[0].ID != "loc-en" {
		t.Fatalf("expected one updated loc-en, got %+v", res.Updated)
	}
	if len(res.Created) != 1 || res.Created[0].ID != "loc-ru" {
		t.Fatalf("expected one created loc-ru, got %+v", res.Created)
	}
}

func TestBulkValidatesBeforeAnyCall(t *testing.T) {
	svc, _ := newService(t, false)
	// No mock expectations: a length violation must short-circuit before any ASC call.
	_, err := svc.BulkUpdateVersionLocalizations(context.Background(), BulkInput{
		VersionID: "ver-1",
		Localizations: []LocaleFields{
			{Locale: "en-US", Fields: VersionFields{Keywords: tooLong(101)}},
		},
	})
	var ve *validate.Error
	if !errors.As(err, &ve) {
		t.Fatalf("expected validation error, got %v", err)
	}
}

func TestBulkDryRunPerformsNoWrites(t *testing.T) {
	svc, _ := newService(t, true)
	// dryRun=true: no GetVersionLocalizations / Update / Create expected.
	out, err := svc.BulkUpdateVersionLocalizations(context.Background(), BulkInput{
		VersionID:     "ver-1",
		Localizations: []LocaleFields{{Locale: "ru", Fields: VersionFields{Description: "ok"}}},
	})
	if err != nil {
		t.Fatalf("dry-run: %v", err)
	}
	if !out.DryRun {
		t.Fatal("expected DryRun outcome")
	}
}

func TestBulkIdempotentCacheHit(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	m := mocks.NewMockascClient(ctrl)
	// fakeStore returns a prior successful result for the same input hash.
	svc := &Service{asc: m, store: &fakeStore{result: `{"versionId":"ver-1"}`}, dryRun: false}

	// No ASC calls expected on a cache hit.
	out, err := svc.BulkUpdateVersionLocalizations(context.Background(), BulkInput{
		VersionID:     "ver-1",
		Localizations: []LocaleFields{{Locale: "ru", Fields: VersionFields{Description: "ok"}}},
	})
	if err != nil {
		t.Fatalf("idempotent: %v", err)
	}
	if !out.Cached {
		t.Fatal("expected cached outcome")
	}
}

func TestUpdateVersionLocalizationRejectsLongKeywords(t *testing.T) {
	svc, _ := newService(t, false)
	_, err := svc.UpdateVersionLocalization(context.Background(), UpdateVersionLocalizationInput{
		LocalizationID: "loc-1",
		Fields:         VersionFields{Keywords: tooLong(200)},
	})
	var ve *validate.Error
	if !errors.As(err, &ve) {
		t.Fatalf("expected validation error, got %v", err)
	}
}

func tooLong(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = 'x'
	}
	return string(b)
}

// fakeStore is a minimal store.Store that simulates an idempotency hit.
type fakeStore struct {
	result string
}

func (f *fakeStore) Enabled() bool { return true }
func (f *fakeStore) LookupOperation(context.Context, string) (store.Operation, bool, error) {
	return store.Operation{Status: store.StatusSuccess, Result: f.result}, true, nil
}
func (f *fakeStore) SaveOperation(context.Context, store.Operation) error { return nil }
func (f *fakeStore) PutCache(context.Context, store.CacheEntry) error      { return nil }
func (f *fakeStore) GetCache(context.Context, string, string, string) (store.CacheEntry, bool, error) {
	return store.CacheEntry{}, false, nil
}

var _ store.Store = (*fakeStore)(nil)
var _ = toolkit.Outcome{}
