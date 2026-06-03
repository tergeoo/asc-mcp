// Package localization implements metadata localization tools for version and
// appInfo localizations, including the bulk update that is the server's primary
// value. Field length limits are validated before any request (spec §8).
package localization

//go:generate go tool mockgen -source=service.go -destination=mocks/mock_ascclient.go -package=mocks ascClient

import (
	"context"
	"fmt"

	"github.com/tergrigoryantc/asc-mcp/internal/shared/asc"
	"github.com/tergrigoryantc/asc-mcp/internal/shared/store"
	"github.com/tergrigoryantc/asc-mcp/internal/shared/toolkit"
	"github.com/tergrigoryantc/asc-mcp/internal/shared/validate"
)

type ascClient interface {
	GetVersionLocalizations(ctx context.Context, versionID string) ([]asc.Resource, error)
	CreateVersionLocalization(ctx context.Context, payload any) (asc.Resource, error)
	UpdateVersionLocalization(ctx context.Context, id string, payload any) (asc.Resource, error)
	UpdateAppInfoLocalization(ctx context.Context, id string, payload any) (asc.Resource, error)
	CreateAppInfoLocalization(ctx context.Context, payload any) (asc.Resource, error)
}

// Service holds localization logic.
type Service struct {
	asc    ascClient
	store  store.Store
	dryRun bool
}

// NewService builds the localization service.
func NewService(d *toolkit.Deps) *Service {
	return &Service{asc: d.ASC, store: d.Store, dryRun: d.DryRun}
}

// VersionLocalization is the trimmed appStoreVersionLocalization representation.
type VersionLocalization struct {
	ID              string `json:"id"`
	Locale          string `json:"locale"`
	Description     string `json:"description,omitempty"`
	Keywords        string `json:"keywords,omitempty"`
	WhatsNew        string `json:"whatsNew,omitempty"`
	PromotionalText string `json:"promotionalText,omitempty"`
	MarketingURL    string `json:"marketingUrl,omitempty"`
	SupportURL      string `json:"supportUrl,omitempty"`
}

type versionLocAttrs struct {
	Locale          string `json:"locale"`
	Description     string `json:"description"`
	Keywords        string `json:"keywords"`
	WhatsNew        string `json:"whatsNew"`
	PromotionalText string `json:"promotionalText"`
	MarketingURL    string `json:"marketingUrl"`
	SupportURL      string `json:"supportUrl"`
}

func toVersionLoc(r asc.Resource) VersionLocalization {
	var a versionLocAttrs
	_ = r.Attr(&a)
	return VersionLocalization{
		ID: r.ID, Locale: a.Locale, Description: a.Description, Keywords: a.Keywords,
		WhatsNew: a.WhatsNew, PromotionalText: a.PromotionalText,
		MarketingURL: a.MarketingURL, SupportURL: a.SupportURL,
	}
}

// ListVersionLocalizations returns all localizations for a version.
func (s *Service) ListVersionLocalizations(ctx context.Context, versionID string) ([]VersionLocalization, error) {
	resources, err := s.asc.GetVersionLocalizations(ctx, versionID)
	if err != nil {
		return nil, err
	}
	out := make([]VersionLocalization, 0, len(resources))
	for _, r := range resources {
		out = append(out, toVersionLoc(r))
	}
	return out, nil
}

// VersionFields are the editable text fields of a version localization. Empty
// fields are left unchanged.
type VersionFields struct {
	Description     string `json:"description,omitempty"`
	Keywords        string `json:"keywords,omitempty"`
	WhatsNew        string `json:"whatsNew,omitempty"`
	PromotionalText string `json:"promotionalText,omitempty"`
	MarketingURL    string `json:"marketingUrl,omitempty"`
	SupportURL      string `json:"supportUrl,omitempty"`
}

// validateVersionFields enforces ASC length limits (spec §8).
func validateVersionFields(locale string, f VersionFields) error {
	b := validate.NewBuilder()
	prefix := ""
	if locale != "" {
		prefix = locale + "."
	}
	b.MaxLen(prefix+"description", f.Description, validate.MaxDescription)
	b.MaxLen(prefix+"keywords", f.Keywords, validate.MaxKeywords)
	b.MaxLen(prefix+"whatsNew", f.WhatsNew, validate.MaxWhatsNew)
	b.MaxLen(prefix+"promotionalText", f.PromotionalText, validate.MaxPromotionalText)
	if e := b.Result(); e != nil {
		return e
	}
	return nil
}

func versionAttrMap(f VersionFields) map[string]any {
	return map[string]any{
		"description":     f.Description,
		"keywords":        f.Keywords,
		"whatsNew":        f.WhatsNew,
		"promotionalText": f.PromotionalText,
		"marketingUrl":    f.MarketingURL,
		"supportUrl":      f.SupportURL,
	}
}

// UpdateVersionLocalizationInput identifies a single localization to update.
type UpdateVersionLocalizationInput struct {
	LocalizationID string        `json:"localizationId"`
	Fields         VersionFields `json:"fields"`
}

// UpdateVersionLocalization edits a single version localization by id.
func (s *Service) UpdateVersionLocalization(ctx context.Context, in UpdateVersionLocalizationInput) (toolkit.Outcome, error) {
	if err := validateVersionFields("", in.Fields); err != nil {
		return toolkit.Outcome{}, err
	}
	payload := asc.Update("appStoreVersionLocalizations", in.LocalizationID, versionAttrMap(in.Fields))
	return toolkit.RunIdempotent(ctx, s.store, s.dryRun,
		toolkit.IdemKey{Tool: "update_version_localization", Input: in},
		func() (any, error) {
			r, err := s.asc.UpdateVersionLocalization(ctx, in.LocalizationID, payload)
			if err != nil {
				return nil, err
			}
			return toVersionLoc(r), nil
		},
	)
}

// CreateVersionLocalizationInput adds a new locale to a version.
type CreateVersionLocalizationInput struct {
	VersionID string        `json:"versionId"`
	Locale    string        `json:"locale"`
	Fields    VersionFields `json:"fields"`
}

// CreateVersionLocalization adds a new locale to a version.
func (s *Service) CreateVersionLocalization(ctx context.Context, in CreateVersionLocalizationInput) (toolkit.Outcome, error) {
	if err := validate.NewBuilder().Required("locale", in.Locale).Result(); err != nil {
		return toolkit.Outcome{}, err
	}
	if err := validateVersionFields(in.Locale, in.Fields); err != nil {
		return toolkit.Outcome{}, err
	}
	attrs := versionAttrMap(in.Fields)
	attrs["locale"] = in.Locale
	payload := asc.Create("appStoreVersionLocalizations", attrs,
		map[string]any{"appStoreVersion": asc.ToOne("appStoreVersions", in.VersionID)})
	return toolkit.RunIdempotent(ctx, s.store, s.dryRun,
		toolkit.IdemKey{Tool: "create_version_localization", VersionID: in.VersionID, Input: in},
		func() (any, error) {
			r, err := s.asc.CreateVersionLocalization(ctx, payload)
			if err != nil {
				return nil, err
			}
			return toVersionLoc(r), nil
		},
	)
}

// LocaleFields pairs a locale with its fields for bulk updates.
type LocaleFields struct {
	Locale string        `json:"locale"`
	Fields VersionFields `json:"fields"`
}

// BulkInput updates many locales of one version in a single call. Locales that
// do not yet exist are created; existing ones are updated.
type BulkInput struct {
	VersionID   string         `json:"versionId"`
	Localizations []LocaleFields `json:"localizations"`
}

// BulkResult reports the per-locale outcome of a bulk update.
type BulkResult struct {
	VersionID string                `json:"versionId"`
	Updated   []VersionLocalization `json:"updated"`
	Created   []VersionLocalization `json:"created"`
}

// BulkUpdateVersionLocalizations updates/creates multiple locales at once. It is
// idempotent over the whole batch and validates every field before any write.
func (s *Service) BulkUpdateVersionLocalizations(ctx context.Context, in BulkInput) (toolkit.Outcome, error) {
	// Validate all locales up front — fail fast before any partial write.
	vb := validate.NewBuilder()
	for _, lf := range in.Localizations {
		vb.Required("locale", lf.Locale)
	}
	if e := vb.Result(); e != nil {
		return toolkit.Outcome{}, e
	}
	for _, lf := range in.Localizations {
		if err := validateVersionFields(lf.Locale, lf.Fields); err != nil {
			return toolkit.Outcome{}, err
		}
	}

	return toolkit.RunIdempotent(ctx, s.store, s.dryRun,
		toolkit.IdemKey{Tool: "bulk_update_version_localizations", VersionID: in.VersionID, Input: in},
		func() (any, error) {
			existing, err := s.asc.GetVersionLocalizations(ctx, in.VersionID)
			if err != nil {
				return nil, err
			}
			byLocale := make(map[string]string, len(existing))
			for _, r := range existing {
				byLocale[toVersionLoc(r).Locale] = r.ID
			}

			result := BulkResult{VersionID: in.VersionID}
			for _, lf := range in.Localizations {
				if id, ok := byLocale[lf.Locale]; ok {
					payload := asc.Update("appStoreVersionLocalizations", id, versionAttrMap(lf.Fields))
					r, err := s.asc.UpdateVersionLocalization(ctx, id, payload)
					if err != nil {
						return nil, fmt.Errorf("locale %s: %w", lf.Locale, err)
					}
					result.Updated = append(result.Updated, toVersionLoc(r))
				} else {
					attrs := versionAttrMap(lf.Fields)
					attrs["locale"] = lf.Locale
					payload := asc.Create("appStoreVersionLocalizations", attrs,
						map[string]any{"appStoreVersion": asc.ToOne("appStoreVersions", in.VersionID)})
					r, err := s.asc.CreateVersionLocalization(ctx, payload)
					if err != nil {
						return nil, fmt.Errorf("locale %s: %w", lf.Locale, err)
					}
					result.Created = append(result.Created, toVersionLoc(r))
				}
			}
			return result, nil
		},
	)
}

// AppInfoFields are the editable appInfo localization fields.
type AppInfoFields struct {
	Name             string `json:"name,omitempty"`
	Subtitle         string `json:"subtitle,omitempty"`
	PrivacyPolicyURL string `json:"privacyPolicyUrl,omitempty"`
}

// CreateAppInfoLocalizationInput adds a new appInfo locale (name, subtitle,
// privacyPolicyUrl).
type CreateAppInfoLocalizationInput struct {
	AppInfoID string        `json:"appInfoId"`
	Locale    string        `json:"locale"`
	Fields    AppInfoFields `json:"fields"`
}

// CreateAppInfoLocalization adds a new locale to an appInfo.
func (s *Service) CreateAppInfoLocalization(ctx context.Context, in CreateAppInfoLocalizationInput) (toolkit.Outcome, error) {
	b := validate.NewBuilder().Required("appInfoId", in.AppInfoID).Required("locale", in.Locale)
	b.MaxLen("name", in.Fields.Name, validate.MaxName)
	b.MaxLen("subtitle", in.Fields.Subtitle, validate.MaxSubtitle)
	if e := b.Result(); e != nil {
		return toolkit.Outcome{}, e
	}
	attrs := map[string]any{
		"locale":           in.Locale,
		"name":             in.Fields.Name,
		"subtitle":         in.Fields.Subtitle,
		"privacyPolicyUrl": in.Fields.PrivacyPolicyURL,
	}
	payload := asc.Create("appInfoLocalizations", attrs,
		map[string]any{"appInfo": asc.ToOne("appInfos", in.AppInfoID)})
	return toolkit.RunIdempotent(ctx, s.store, s.dryRun,
		toolkit.IdemKey{Tool: "create_app_info_localization", Input: in},
		func() (any, error) {
			r, err := s.asc.CreateAppInfoLocalization(ctx, payload)
			if err != nil {
				return nil, err
			}
			return map[string]any{"id": r.ID, "locale": in.Locale, "name": in.Fields.Name, "subtitle": in.Fields.Subtitle}, nil
		},
	)
}

// UpdateAppInfoLocalizationInput identifies an appInfo localization to update.
type UpdateAppInfoLocalizationInput struct {
	LocalizationID string        `json:"localizationId"`
	Fields         AppInfoFields `json:"fields"`
}

// UpdateAppInfoLocalization edits name, subtitle and privacyPolicyUrl.
func (s *Service) UpdateAppInfoLocalization(ctx context.Context, in UpdateAppInfoLocalizationInput) (toolkit.Outcome, error) {
	b := validate.NewBuilder()
	b.MaxLen("name", in.Fields.Name, validate.MaxName)
	b.MaxLen("subtitle", in.Fields.Subtitle, validate.MaxSubtitle)
	if e := b.Result(); e != nil {
		return toolkit.Outcome{}, e
	}
	payload := asc.Update("appInfoLocalizations", in.LocalizationID, map[string]any{
		"name":             in.Fields.Name,
		"subtitle":         in.Fields.Subtitle,
		"privacyPolicyUrl": in.Fields.PrivacyPolicyURL,
	})
	return toolkit.RunIdempotent(ctx, s.store, s.dryRun,
		toolkit.IdemKey{Tool: "update_app_info_localization", Input: in},
		func() (any, error) {
			r, err := s.asc.UpdateAppInfoLocalization(ctx, in.LocalizationID, payload)
			if err != nil {
				return nil, err
			}
			var a struct {
				Locale           string `json:"locale"`
				Name             string `json:"name"`
				Subtitle         string `json:"subtitle"`
				PrivacyPolicyURL string `json:"privacyPolicyUrl"`
			}
			_ = r.Attr(&a)
			return map[string]any{
				"id": r.ID, "locale": a.Locale, "name": a.Name,
				"subtitle": a.Subtitle, "privacyPolicyUrl": a.PrivacyPolicyURL,
			}, nil
		},
	)
}
