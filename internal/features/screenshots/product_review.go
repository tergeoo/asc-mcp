package screenshots

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/tergrigoryantc/asc-mcp/internal/shared/asc"
	"github.com/tergrigoryantc/asc-mcp/internal/shared/toolkit"
	"github.com/tergrigoryantc/asc-mcp/internal/shared/validate"
)

// Product kinds for review screenshots.
const (
	KindIAP          = "iap"
	KindSubscription = "subscription"
)

// ProductReviewInput holds upload_product_review_screenshot parameters.
type ProductReviewInput struct {
	ProductID   string `json:"productId"`
	ProductType string `json:"productType"` // "iap" or "subscription"
	FilePath    string `json:"filePath"`
}

// UploadProductReviewScreenshot uploads the App Store review screenshot for an
// IAP or subscription (reserve -> upload chunks -> commit with MD5). This clears
// the missing-review-screenshot part of a product's MISSING_METADATA state and
// does not require the product to load in the app.
func (s *Service) UploadProductReviewScreenshot(ctx context.Context, in ProductReviewInput) (toolkit.Outcome, error) {
	b := validate.NewBuilder().
		Required("productId", in.ProductID).
		Required("productType", in.ProductType).
		Required("filePath", in.FilePath)
	if e := b.Result(); e != nil {
		return toolkit.Outcome{}, e
	}
	if in.ProductType != KindIAP && in.ProductType != KindSubscription {
		return toolkit.Outcome{}, &validate.Error{Fields: []validate.FieldError{{
			Field: "productType", Message: "must be 'iap' or 'subscription'",
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

	idem := map[string]any{
		"productId": in.ProductID, "productType": in.ProductType,
		"fileName": fileName, "fileSize": len(data), "checksum": checksum,
	}
	return toolkit.RunIdempotent(ctx, s.store, s.dryRun,
		toolkit.IdemKey{Tool: "upload_product_review_screenshot", Input: idem},
		func() (any, error) {
			// Replace semantics: delete any existing (possibly FAILED) review
			// screenshot first, otherwise ASC rejects the new one as "already exists".
			if err := s.deleteExistingReview(ctx, in.ProductType, in.ProductID); err != nil {
				return nil, err
			}

			reserve, commit, payload := s.reviewOps(in.ProductType, in.ProductID, fileName, len(data))

			reserved, err := reserve(ctx, payload)
			if err != nil {
				return nil, err
			}
			var ra screenshotAttrs
			if err := reserved.Attr(&ra); err != nil {
				return nil, fmt.Errorf("decode reservation: %w", err)
			}
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
			committed, err := commit(ctx, reserved.ID, map[string]any{"uploaded": true, "sourceFileChecksum": checksum})
			if err != nil {
				return nil, err
			}
			return map[string]any{
				"id": committed.ID, "productId": in.ProductID, "productType": in.ProductType,
				"fileName": fileName, "sourceFileChecksum": checksum,
			}, nil
		},
	)
}

// reviewOps returns the reserve/commit functions and reservation payload for the
// given product kind.
func (s *Service) reviewOps(kind, productID, fileName string, size int) (
	reserve func(context.Context, any) (asc.Resource, error),
	commit func(context.Context, string, any) (asc.Resource, error),
	payload map[string]any,
) {
	attrs := map[string]any{"fileName": fileName, "fileSize": size}
	if kind == KindSubscription {
		return s.asc.ReserveSubscriptionReviewScreenshot, s.commitSubscription,
			asc.Create("subscriptionAppStoreReviewScreenshots", attrs,
				map[string]any{"subscription": asc.ToOne("subscriptions", productID)})
	}
	return s.asc.ReserveIAPReviewScreenshot, s.commitIAP,
		asc.Create("inAppPurchaseAppStoreReviewScreenshots", attrs,
			map[string]any{"inAppPurchaseV2": asc.ToOne("inAppPurchases", productID)})
}

// deleteExistingReview removes any current review screenshot on the product so
// a fresh one can be uploaded (ASC allows only one and rejects duplicates).
func (s *Service) deleteExistingReview(ctx context.Context, kind, productID string) error {
	if kind == KindSubscription {
		id, err := s.asc.GetSubscriptionReviewScreenshot(ctx, productID)
		if err != nil || id == "" {
			return err
		}
		return s.asc.DeleteSubscriptionReviewScreenshot(ctx, id)
	}
	id, err := s.asc.GetIAPReviewScreenshot(ctx, productID)
	if err != nil || id == "" {
		return err
	}
	return s.asc.DeleteIAPReviewScreenshot(ctx, id)
}

func (s *Service) commitIAP(ctx context.Context, id string, attrs any) (asc.Resource, error) {
	return s.asc.CommitIAPReviewScreenshot(ctx, id, asc.Update("inAppPurchaseAppStoreReviewScreenshots", id, attrs.(map[string]any)))
}

func (s *Service) commitSubscription(ctx context.Context, id string, attrs any) (asc.Resource, error) {
	return s.asc.CommitSubscriptionReviewScreenshot(ctx, id, asc.Update("subscriptionAppStoreReviewScreenshots", id, attrs.(map[string]any)))
}
