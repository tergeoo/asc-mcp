// Package screenshots implements the screenshot tools. upload_screenshot is a
// multi-step operation (reserve -> upload chunks -> commit with MD5) executed
// inside a single tool call, per spec §5.5.
package screenshots

//go:generate go tool mockgen -source=service.go -destination=mocks/mock_ascclient.go -package=mocks ascClient

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/tergrigoryantc/asc-mcp/internal/shared/asc"
	"github.com/tergrigoryantc/asc-mcp/internal/shared/store"
	"github.com/tergrigoryantc/asc-mcp/internal/shared/toolkit"
	"github.com/tergrigoryantc/asc-mcp/internal/shared/validate"
)

type ascClient interface {
	GetVersionLocScreenshotSets(ctx context.Context, versionLocID string) ([]asc.Resource, error)
	CreateScreenshotSet(ctx context.Context, payload any) (asc.Resource, error)
	DeleteScreenshotSet(ctx context.Context, id string) error
	GetSetScreenshots(ctx context.Context, setID string) ([]asc.Resource, error)
	ReserveScreenshot(ctx context.Context, payload any) (asc.Resource, error)
	CommitScreenshot(ctx context.Context, id string, payload any) (asc.Resource, error)
	DeleteScreenshot(ctx context.Context, id string) error
	UploadChunk(ctx context.Context, method, url string, headers map[string]string, data []byte) error

	// Product review screenshots (reserve/commit + replace existing).
	ReserveIAPReviewScreenshot(ctx context.Context, payload any) (asc.Resource, error)
	CommitIAPReviewScreenshot(ctx context.Context, id string, payload any) (asc.Resource, error)
	ReserveSubscriptionReviewScreenshot(ctx context.Context, payload any) (asc.Resource, error)
	CommitSubscriptionReviewScreenshot(ctx context.Context, id string, payload any) (asc.Resource, error)
	GetIAPReviewScreenshot(ctx context.Context, iapID string) (string, error)
	GetSubscriptionReviewScreenshot(ctx context.Context, subID string) (string, error)
	DeleteIAPReviewScreenshot(ctx context.Context, id string) error
	DeleteSubscriptionReviewScreenshot(ctx context.Context, id string) error
}

// Service holds screenshot logic.
type Service struct {
	asc    ascClient
	store  store.Store
	dryRun bool
}

// NewService builds the screenshots service.
func NewService(d *toolkit.Deps) *Service {
	return &Service{asc: d.ASC, store: d.Store, dryRun: d.DryRun}
}

// allowedImageExt are the formats App Store Connect accepts for screenshots.
var allowedImageExt = map[string]bool{".png": true, ".jpg": true, ".jpeg": true}

// uploadOperation mirrors ASC's reservation upload instructions.
type uploadOperation struct {
	Method         string `json:"method"`
	URL            string `json:"url"`
	Length         int    `json:"length"`
	Offset         int    `json:"offset"`
	RequestHeaders []struct {
		Name  string `json:"name"`
		Value string `json:"value"`
	} `json:"requestHeaders"`
}

type screenshotAttrs struct {
	FileName          string            `json:"fileName"`
	FileSize          int               `json:"fileSize"`
	SourceFileChecksum string           `json:"sourceFileChecksum"`
	AssetDeliveryState map[string]any   `json:"assetDeliveryState"`
	UploadOperations  []uploadOperation `json:"uploadOperations"`
}

type setAttrs struct {
	ScreenshotDisplayType string `json:"screenshotDisplayType"`
}

// ScreenshotSet is the trimmed representation returned by list_screenshot_sets.
type ScreenshotSet struct {
	ID          string       `json:"id"`
	DisplayType string       `json:"displayType"`
	Screenshots []Screenshot `json:"screenshots"`
}

// Screenshot is one screenshot in a set.
type Screenshot struct {
	ID       string `json:"id"`
	FileName string `json:"fileName"`
}

// ListSets returns the screenshot sets (and their screenshots) for a version
// localization.
func (s *Service) ListSets(ctx context.Context, versionLocID string) ([]ScreenshotSet, error) {
	sets, err := s.asc.GetVersionLocScreenshotSets(ctx, versionLocID)
	if err != nil {
		return nil, err
	}
	out := make([]ScreenshotSet, 0, len(sets))
	for _, set := range sets {
		var sa setAttrs
		_ = set.Attr(&sa)
		entry := ScreenshotSet{ID: set.ID, DisplayType: sa.ScreenshotDisplayType, Screenshots: []Screenshot{}}
		shots, err := s.asc.GetSetScreenshots(ctx, set.ID)
		if err != nil {
			return nil, err
		}
		for _, sh := range shots {
			var a screenshotAttrs
			_ = sh.Attr(&a)
			entry.Screenshots = append(entry.Screenshots, Screenshot{ID: sh.ID, FileName: a.FileName})
		}
		out = append(out, entry)
	}
	return out, nil
}

// UploadInput holds upload_screenshot parameters.
type UploadInput struct {
	VersionLocalizationID string `json:"versionLocalizationId"`
	DisplayType           string `json:"displayType"`
	FilePath              string `json:"filePath"`
}

// Upload reserves, uploads and commits a screenshot in one call.
func (s *Service) Upload(ctx context.Context, in UploadInput) (toolkit.Outcome, error) {
	// Validate inputs before any reservation (spec §8).
	b := validate.NewBuilder().
		Required("versionLocalizationId", in.VersionLocalizationID).
		Required("displayType", in.DisplayType).
		Required("filePath", in.FilePath)
	if e := b.Result(); e != nil {
		return toolkit.Outcome{}, e
	}
	if !strings.HasPrefix(in.DisplayType, "APP_") {
		return toolkit.Outcome{}, &validate.Error{Fields: []validate.FieldError{{
			Field: "displayType", Message: "must be an APP_* screenshot display type",
		}}}
	}
	ext := strings.ToLower(filepath.Ext(in.FilePath))
	if !allowedImageExt[ext] {
		return toolkit.Outcome{}, &validate.Error{Fields: []validate.FieldError{{
			Field: "filePath", Message: fmt.Sprintf("unsupported image format %q (allowed: .png, .jpg, .jpeg)", ext),
		}}}
	}
	data, err := os.ReadFile(in.FilePath)
	if err != nil {
		return toolkit.Outcome{}, fmt.Errorf("read screenshot file: %w", err)
	}
	fileName := filepath.Base(in.FilePath)
	sum := md5.Sum(data)
	checksum := hex.EncodeToString(sum[:])

	idemInput := map[string]any{
		"versionLocalizationId": in.VersionLocalizationID,
		"displayType":           in.DisplayType,
		"fileName":              fileName,
		"fileSize":              len(data),
		"checksum":              checksum,
	}

	return toolkit.RunIdempotent(ctx, s.store, s.dryRun,
		toolkit.IdemKey{Tool: "upload_screenshot", Input: idemInput},
		func() (any, error) {
			setID, err := s.findOrCreateSet(ctx, in.VersionLocalizationID, in.DisplayType)
			if err != nil {
				return nil, err
			}
			return s.reserveUploadCommit(ctx, setID, fileName, data, checksum)
		},
	)
}

func (s *Service) findOrCreateSet(ctx context.Context, versionLocID, displayType string) (string, error) {
	sets, err := s.asc.GetVersionLocScreenshotSets(ctx, versionLocID)
	if err != nil {
		return "", err
	}
	for _, set := range sets {
		var sa setAttrs
		_ = set.Attr(&sa)
		if sa.ScreenshotDisplayType == displayType {
			return set.ID, nil
		}
	}
	payload := asc.Create("appScreenshotSets",
		map[string]any{"screenshotDisplayType": displayType},
		map[string]any{"appStoreVersionLocalization": asc.ToOne("appStoreVersionLocalizations", versionLocID)},
	)
	created, err := s.asc.CreateScreenshotSet(ctx, payload)
	if err != nil {
		return "", err
	}
	return created.ID, nil
}

func (s *Service) reserveUploadCommit(ctx context.Context, setID, fileName string, data []byte, checksum string) (any, error) {
	// 1. Reserve.
	reservePayload := asc.Create("appScreenshots",
		map[string]any{"fileName": fileName, "fileSize": len(data)},
		map[string]any{"appScreenshotSet": asc.ToOne("appScreenshotSets", setID)},
	)
	reserved, err := s.asc.ReserveScreenshot(ctx, reservePayload)
	if err != nil {
		return nil, err
	}
	var ra screenshotAttrs
	if err := reserved.Attr(&ra); err != nil {
		return nil, fmt.Errorf("decode reservation: %w", err)
	}

	// 2. Upload each chunk.
	for _, op := range ra.UploadOperations {
		end := op.Offset + op.Length
		if op.Offset < 0 || end > len(data) {
			return nil, fmt.Errorf("invalid upload operation bounds [%d:%d] for %d-byte file", op.Offset, end, len(data))
		}
		headers := make(map[string]string, len(op.RequestHeaders))
		for _, h := range op.RequestHeaders {
			headers[h.Name] = h.Value
		}
		if err := s.asc.UploadChunk(ctx, op.Method, op.URL, headers, data[op.Offset:end]); err != nil {
			return nil, fmt.Errorf("upload chunk at offset %d: %w", op.Offset, err)
		}
	}

	// 3. Commit with MD5.
	commitPayload := asc.Update("appScreenshots", reserved.ID, map[string]any{
		"uploaded":           true,
		"sourceFileChecksum": checksum,
	})
	committed, err := s.asc.CommitScreenshot(ctx, reserved.ID, commitPayload)
	if err != nil {
		return nil, err
	}
	var ca screenshotAttrs
	_ = committed.Attr(&ca)
	return map[string]any{
		"id":                 committed.ID,
		"setId":              setID,
		"fileName":           ca.FileName,
		"fileSize":           ca.FileSize,
		"sourceFileChecksum": checksum,
		"assetDeliveryState": ca.AssetDeliveryState,
	}, nil
}

// Delete removes a screenshot or a whole set.
type DeleteInput struct {
	ID   string `json:"id"`
	Kind string `json:"kind"` // "screenshot" (default) or "set"
}

// Delete removes a screenshot or set by id.
func (s *Service) Delete(ctx context.Context, in DeleteInput) (toolkit.Outcome, error) {
	kind := in.Kind
	if kind == "" {
		kind = "screenshot"
	}
	return toolkit.RunIdempotent(ctx, s.store, s.dryRun,
		toolkit.IdemKey{Tool: "delete_screenshot", Input: in},
		func() (any, error) {
			switch kind {
			case "set":
				if err := s.asc.DeleteScreenshotSet(ctx, in.ID); err != nil {
					return nil, err
				}
			case "screenshot":
				if err := s.asc.DeleteScreenshot(ctx, in.ID); err != nil {
					return nil, err
				}
			default:
				return nil, fmt.Errorf("unknown kind %q (use 'screenshot' or 'set')", kind)
			}
			return map[string]any{"deleted": in.ID, "kind": kind}, nil
		},
	)
}
