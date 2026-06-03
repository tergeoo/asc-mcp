// Package submission implements review submission tools: creating a review
// submission, attaching a version, submitting for review and reading status.
package submission

//go:generate go tool mockgen -source=service.go -destination=mocks/mock_ascclient.go -package=mocks ascClient

import (
	"context"

	"github.com/tergrigoryantc/asc-mcp/internal/shared/asc"
	"github.com/tergrigoryantc/asc-mcp/internal/shared/store"
	"github.com/tergrigoryantc/asc-mcp/internal/shared/toolkit"
	"github.com/tergrigoryantc/asc-mcp/internal/shared/validate"
)

type ascClient interface {
	CreateReviewSubmission(ctx context.Context, payload any) (asc.Resource, error)
	GetReviewSubmission(ctx context.Context, id string) (asc.Resource, error)
	UpdateReviewSubmission(ctx context.Context, id string, payload any) (asc.Resource, error)
	CreateReviewSubmissionItem(ctx context.Context, payload any) (asc.Resource, error)
}

// Service holds submission logic.
type Service struct {
	asc    ascClient
	store  store.Store
	dryRun bool
}

// NewService builds the submission service.
func NewService(d *toolkit.Deps) *Service {
	return &Service{asc: d.ASC, store: d.Store, dryRun: d.DryRun}
}

type submissionAttrs struct {
	Platform      string `json:"platform"`
	State         string `json:"state"`
	SubmittedDate string `json:"submittedDate"`
}

func toSubmission(r asc.Resource) map[string]any {
	var a submissionAttrs
	_ = r.Attr(&a)
	return map[string]any{
		"id": r.ID, "platform": a.Platform, "state": a.State, "submittedDate": a.SubmittedDate,
	}
}

// Create makes a new review submission for an app+platform.
func (s *Service) Create(ctx context.Context, appID, platform string) (toolkit.Outcome, error) {
	if e := validate.NewBuilder().Required("appId", appID).Required("platform", platform).Result(); e != nil {
		return toolkit.Outcome{}, e
	}
	payload := asc.Create("reviewSubmissions",
		map[string]any{"platform": platform},
		map[string]any{"app": asc.ToOne("apps", appID)},
	)
	return toolkit.RunIdempotent(ctx, s.store, s.dryRun,
		toolkit.IdemKey{Tool: "create_review_submission", AppID: appID, Input: map[string]any{"appId": appID, "platform": platform}},
		func() (any, error) {
			r, err := s.asc.CreateReviewSubmission(ctx, payload)
			if err != nil {
				return nil, err
			}
			return toSubmission(r), nil
		},
	)
}

// AddVersion attaches a version to a submission as a reviewSubmissionItem.
func (s *Service) AddVersion(ctx context.Context, submissionID, versionID string) (toolkit.Outcome, error) {
	if e := validate.NewBuilder().Required("submissionId", submissionID).Required("versionId", versionID).Result(); e != nil {
		return toolkit.Outcome{}, e
	}
	payload := asc.Create("reviewSubmissionItems", nil, map[string]any{
		"reviewSubmission": asc.ToOne("reviewSubmissions", submissionID),
		"appStoreVersion":  asc.ToOne("appStoreVersions", versionID),
	})
	return toolkit.RunIdempotent(ctx, s.store, s.dryRun,
		toolkit.IdemKey{Tool: "add_version_to_submission", VersionID: versionID, Input: map[string]any{"submissionId": submissionID, "versionId": versionID}},
		func() (any, error) {
			r, err := s.asc.CreateReviewSubmissionItem(ctx, payload)
			if err != nil {
				return nil, err
			}
			return map[string]any{"id": r.ID, "submissionId": submissionID, "versionId": versionID}, nil
		},
	)
}

// Submit finalizes a submission by setting submitted=true.
func (s *Service) Submit(ctx context.Context, submissionID string) (toolkit.Outcome, error) {
	if e := validate.NewBuilder().Required("submissionId", submissionID).Result(); e != nil {
		return toolkit.Outcome{}, e
	}
	payload := asc.Update("reviewSubmissions", submissionID, map[string]any{"submitted": true})
	return toolkit.RunIdempotent(ctx, s.store, s.dryRun,
		toolkit.IdemKey{Tool: "submit_for_review", Input: map[string]any{"submissionId": submissionID}},
		func() (any, error) {
			r, err := s.asc.UpdateReviewSubmission(ctx, submissionID, payload)
			if err != nil {
				return nil, err
			}
			return toSubmission(r), nil
		},
	)
}

// Status returns the current state of a review submission.
func (s *Service) Status(ctx context.Context, submissionID string) (map[string]any, error) {
	r, err := s.asc.GetReviewSubmission(ctx, submissionID)
	if err != nil {
		return nil, err
	}
	return toSubmission(r), nil
}
