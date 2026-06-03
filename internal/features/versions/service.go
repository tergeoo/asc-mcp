// Package versions implements version discovery and lifecycle tools:
// list_versions, create_version and update_version.
package versions

//go:generate go tool mockgen -source=service.go -destination=mocks/mock_ascclient.go -package=mocks ascClient

import (
	"context"

	"github.com/tergeoo/asc-mcp/internal/shared/asc"
	"github.com/tergeoo/asc-mcp/internal/shared/store"
	"github.com/tergeoo/asc-mcp/internal/shared/toolkit"
)

type ascClient interface {
	GetAppVersions(ctx context.Context, appID string) ([]asc.Resource, error)
	GetVersion(ctx context.Context, id string) (asc.Resource, error)
	CreateVersion(ctx context.Context, payload any) (asc.Resource, error)
	UpdateVersion(ctx context.Context, id string, payload any) (asc.Resource, error)
}

// Service holds version logic.
type Service struct {
	asc    ascClient
	store  store.Store
	dryRun bool
}

// NewService builds the versions service.
func NewService(d *toolkit.Deps) *Service {
	return &Service{asc: d.ASC, store: d.Store, dryRun: d.DryRun}
}

// Version is the trimmed representation of an appStoreVersion.
type Version struct {
	ID                 string `json:"id"`
	VersionString      string `json:"versionString"`
	Platform           string `json:"platform,omitempty"`
	AppStoreState      string `json:"appStoreState,omitempty"`
	ReleaseType        string `json:"releaseType,omitempty"`
	EarliestReleaseDate string `json:"earliestReleaseDate,omitempty"`
	Copyright          string `json:"copyright,omitempty"`
}

type versionAttributes struct {
	VersionString       string `json:"versionString"`
	Platform            string `json:"platform"`
	AppStoreState       string `json:"appStoreState"`
	ReleaseType         string `json:"releaseType"`
	EarliestReleaseDate string `json:"earliestReleaseDate"`
	Copyright           string `json:"copyright"`
}

func toVersion(r asc.Resource) Version {
	var a versionAttributes
	_ = r.Attr(&a)
	return Version{
		ID: r.ID, VersionString: a.VersionString, Platform: a.Platform,
		AppStoreState: a.AppStoreState, ReleaseType: a.ReleaseType,
		EarliestReleaseDate: a.EarliestReleaseDate, Copyright: a.Copyright,
	}
}

// List returns an app's versions, optionally filtered by appStoreState.
func (s *Service) List(ctx context.Context, appID, stateFilter string) ([]Version, error) {
	resources, err := s.asc.GetAppVersions(ctx, appID)
	if err != nil {
		return nil, err
	}
	out := make([]Version, 0, len(resources))
	for _, r := range resources {
		v := toVersion(r)
		if stateFilter != "" && v.AppStoreState != stateFilter {
			continue
		}
		out = append(out, v)
	}
	return out, nil
}

// CreateInput holds create_version parameters.
type CreateInput struct {
	AppID         string `json:"appId"`
	VersionString string `json:"versionString"`
	Platform      string `json:"platform"`
}

// Create makes a new version for an app.
func (s *Service) Create(ctx context.Context, in CreateInput) (toolkit.Outcome, error) {
	payload := asc.Create("appStoreVersions",
		map[string]any{"platform": in.Platform, "versionString": in.VersionString},
		map[string]any{"app": asc.ToOne("apps", in.AppID)},
	)
	return toolkit.RunIdempotent(ctx, s.store, s.dryRun,
		toolkit.IdemKey{Tool: "create_version", AppID: in.AppID, Input: in},
		func() (any, error) {
			r, err := s.asc.CreateVersion(ctx, payload)
			if err != nil {
				return nil, err
			}
			return toVersion(r), nil
		},
	)
}

// UpdateInput holds update_version parameters.
type UpdateInput struct {
	VersionID           string `json:"versionId"`
	ReleaseType         string `json:"releaseType,omitempty"`
	EarliestReleaseDate string `json:"earliestReleaseDate,omitempty"`
	Copyright           string `json:"copyright,omitempty"`
}

// Update edits releaseType, earliestReleaseDate and copyright.
func (s *Service) Update(ctx context.Context, in UpdateInput) (toolkit.Outcome, error) {
	payload := asc.Update("appStoreVersions", in.VersionID, map[string]any{
		"releaseType":         in.ReleaseType,
		"earliestReleaseDate": in.EarliestReleaseDate,
		"copyright":           in.Copyright,
	})
	return toolkit.RunIdempotent(ctx, s.store, s.dryRun,
		toolkit.IdemKey{Tool: "update_version", VersionID: in.VersionID, Input: in},
		func() (any, error) {
			r, err := s.asc.UpdateVersion(ctx, in.VersionID, payload)
			if err != nil {
				return nil, err
			}
			return toVersion(r), nil
		},
	)
}
