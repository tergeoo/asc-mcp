package localization

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/tergrigoryantc/asc-mcp/internal/shared/toolkit"
)

var errMissingLocalizations = errors.New("localizations array is required")

// Register wires localization tools onto the MCP server.
func Register(s *server.MCPServer, d *toolkit.Deps) {
	svc := NewService(d)
	addGet(s, svc)
	addUpdateVersionLoc(s, svc)
	addCreateVersionLoc(s, svc)
	addBulk(s, svc)
	addUpdateAppInfoLoc(s, svc)
	addCreateAppInfoLoc(s, svc)
}

func addCreateAppInfoLoc(s *server.MCPServer, svc *Service) {
	s.AddTool(
		mcp.NewTool("create_app_info_localization",
			mcp.WithDescription("Add a new appInfo locale (name, subtitle, privacyPolicyUrl) to an app."),
			mcp.WithString("appInfoId", mcp.Required(), mcp.Description("appInfo id (from get_app_info).")),
			mcp.WithString("locale", mcp.Required(), mcp.Description("Locale code, e.g. ru, en-US.")),
			mcp.WithString("name", mcp.Description("App name (<=30 chars).")),
			mcp.WithString("subtitle", mcp.Description("Subtitle (<=30 chars).")),
			mcp.WithString("privacyPolicyUrl", mcp.Description("Privacy policy URL.")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			appInfoID, err := req.RequireString("appInfoId")
			if err != nil {
				return toolkit.Fail(err)
			}
			locale, err := req.RequireString("locale")
			if err != nil {
				return toolkit.Fail(err)
			}
			out, err := svc.CreateAppInfoLocalization(ctx, CreateAppInfoLocalizationInput{
				AppInfoID: appInfoID, Locale: locale,
				Fields: AppInfoFields{
					Name:             req.GetString("name", ""),
					Subtitle:         req.GetString("subtitle", ""),
					PrivacyPolicyURL: req.GetString("privacyPolicyUrl", ""),
				},
			})
			if err != nil {
				return toolkit.Fail(err)
			}
			return toolkit.Result(out)
		},
	)
}

func addGet(s *server.MCPServer, svc *Service) {
	s.AddTool(
		mcp.NewTool("get_version_localizations",
			mcp.WithDescription("List all localizations of a specific App Store version."),
			mcp.WithString("versionId", mcp.Required(), mcp.Description("appStoreVersion id.")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			versionID, err := req.RequireString("versionId")
			if err != nil {
				return toolkit.Fail(err)
			}
			out, err := svc.ListVersionLocalizations(ctx, versionID)
			if err != nil {
				return toolkit.Fail(err)
			}
			return toolkit.Result(map[string]any{"localizations": out})
		},
	)
}

// versionFieldOptions are the shared version localization text field params.
func versionFieldOptions() []mcp.ToolOption {
	return []mcp.ToolOption{
		mcp.WithString("description", mcp.Description("Description (<=4000 chars).")),
		mcp.WithString("keywords", mcp.Description("Comma-separated keywords (<=100 chars).")),
		mcp.WithString("whatsNew", mcp.Description("What's New text (<=4000 chars).")),
		mcp.WithString("promotionalText", mcp.Description("Promotional text (<=170 chars).")),
		mcp.WithString("marketingUrl", mcp.Description("Marketing URL.")),
		mcp.WithString("supportUrl", mcp.Description("Support URL.")),
	}
}

func versionFieldsFromReq(req mcp.CallToolRequest) VersionFields {
	return VersionFields{
		Description:     req.GetString("description", ""),
		Keywords:        req.GetString("keywords", ""),
		WhatsNew:        req.GetString("whatsNew", ""),
		PromotionalText: req.GetString("promotionalText", ""),
		MarketingURL:    req.GetString("marketingUrl", ""),
		SupportURL:      req.GetString("supportUrl", ""),
	}
}

func addUpdateVersionLoc(s *server.MCPServer, svc *Service) {
	opts := append([]mcp.ToolOption{
		mcp.WithDescription("Update text fields of a single version localization, identified by its id."),
		mcp.WithString("localizationId", mcp.Required(), mcp.Description("appStoreVersionLocalization id.")),
	}, versionFieldOptions()...)
	s.AddTool(
		mcp.NewTool("update_version_localization", opts...),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			id, err := req.RequireString("localizationId")
			if err != nil {
				return toolkit.Fail(err)
			}
			out, err := svc.UpdateVersionLocalization(ctx, UpdateVersionLocalizationInput{
				LocalizationID: id, Fields: versionFieldsFromReq(req),
			})
			if err != nil {
				return toolkit.Fail(err)
			}
			return toolkit.Result(out)
		},
	)
}

func addCreateVersionLoc(s *server.MCPServer, svc *Service) {
	opts := append([]mcp.ToolOption{
		mcp.WithDescription("Add a new locale to a version with optional text fields."),
		mcp.WithString("versionId", mcp.Required(), mcp.Description("appStoreVersion id.")),
		mcp.WithString("locale", mcp.Required(), mcp.Description("Locale code, e.g. en-US, ru, uz.")),
	}, versionFieldOptions()...)
	s.AddTool(
		mcp.NewTool("create_version_localization", opts...),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			versionID, err := req.RequireString("versionId")
			if err != nil {
				return toolkit.Fail(err)
			}
			locale, err := req.RequireString("locale")
			if err != nil {
				return toolkit.Fail(err)
			}
			out, err := svc.CreateVersionLocalization(ctx, CreateVersionLocalizationInput{
				VersionID: versionID, Locale: locale, Fields: versionFieldsFromReq(req),
			})
			if err != nil {
				return toolkit.Fail(err)
			}
			return toolkit.Result(out)
		},
	)
}

func addBulk(s *server.MCPServer, svc *Service) {
	s.AddTool(
		mcp.NewTool("bulk_update_version_localizations",
			mcp.WithDescription("Update or create many locales of one version in a single call. "+
				"Each item has a locale and a fields object (description, keywords, whatsNew, "+
				"promotionalText, marketingUrl, supportUrl). Locales that do not exist are created."),
			mcp.WithString("versionId", mcp.Required(), mcp.Description("appStoreVersion id.")),
			mcp.WithArray("localizations", mcp.Required(),
				mcp.Description("Array of {locale, fields} objects."),
				mcp.Items(map[string]any{
					"type": "object",
					"properties": map[string]any{
						"locale": map[string]any{"type": "string"},
						"fields": map[string]any{
							"type": "object",
							"properties": map[string]any{
								"description":     map[string]any{"type": "string"},
								"keywords":        map[string]any{"type": "string"},
								"whatsNew":        map[string]any{"type": "string"},
								"promotionalText": map[string]any{"type": "string"},
								"marketingUrl":    map[string]any{"type": "string"},
								"supportUrl":      map[string]any{"type": "string"},
							},
						},
					},
					"required": []string{"locale"},
				}),
			),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			versionID, err := req.RequireString("versionId")
			if err != nil {
				return toolkit.Fail(err)
			}
			raw, ok := req.GetArguments()["localizations"]
			if !ok {
				return toolkit.Fail(errMissingLocalizations)
			}
			items, err := decodeLocaleFields(raw)
			if err != nil {
				return toolkit.Fail(err)
			}
			out, err := svc.BulkUpdateVersionLocalizations(ctx, BulkInput{VersionID: versionID, Localizations: items})
			if err != nil {
				return toolkit.Fail(err)
			}
			return toolkit.Result(out)
		},
	)
}

func addUpdateAppInfoLoc(s *server.MCPServer, svc *Service) {
	s.AddTool(
		mcp.NewTool("update_app_info_localization",
			mcp.WithDescription("Update an appInfo localization (name, subtitle, privacyPolicyUrl) by its id."),
			mcp.WithString("localizationId", mcp.Required(), mcp.Description("appInfoLocalization id.")),
			mcp.WithString("name", mcp.Description("App name (<=30 chars).")),
			mcp.WithString("subtitle", mcp.Description("Subtitle (<=30 chars).")),
			mcp.WithString("privacyPolicyUrl", mcp.Description("Privacy policy URL.")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			id, err := req.RequireString("localizationId")
			if err != nil {
				return toolkit.Fail(err)
			}
			out, err := svc.UpdateAppInfoLocalization(ctx, UpdateAppInfoLocalizationInput{
				LocalizationID: id,
				Fields: AppInfoFields{
					Name:             req.GetString("name", ""),
					Subtitle:         req.GetString("subtitle", ""),
					PrivacyPolicyURL: req.GetString("privacyPolicyUrl", ""),
				},
			})
			if err != nil {
				return toolkit.Fail(err)
			}
			return toolkit.Result(out)
		},
	)
}

// decodeLocaleFields converts the raw JSON array argument into typed items.
func decodeLocaleFields(raw any) ([]LocaleFields, error) {
	b, err := json.Marshal(raw)
	if err != nil {
		return nil, err
	}
	var items []LocaleFields
	if err := json.Unmarshal(b, &items); err != nil {
		return nil, err
	}
	return items, nil
}
