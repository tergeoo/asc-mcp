package versions

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/tergrigoryantc/asc-mcp/internal/shared/toolkit"
)

// Register wires version tools onto the MCP server.
func Register(s *server.MCPServer, d *toolkit.Deps) {
	svc := NewService(d)
	addList(s, svc)
	addCreate(s, svc)
	addUpdate(s, svc)
}

func addList(s *server.MCPServer, svc *Service) {
	s.AddTool(
		mcp.NewTool("list_versions",
			mcp.WithDescription("List an app's App Store versions, optionally filtered by appStoreState."),
			mcp.WithString("appId", mcp.Required(), mcp.Description("App Store Connect app id.")),
			mcp.WithString("appStoreState", mcp.Description("Optional appStoreState filter, e.g. PREPARE_FOR_SUBMISSION.")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			appID, err := req.RequireString("appId")
			if err != nil {
				return toolkit.Fail(err)
			}
			out, err := svc.List(ctx, appID, req.GetString("appStoreState", ""))
			if err != nil {
				return toolkit.Fail(err)
			}
			return toolkit.Result(map[string]any{"versions": out})
		},
	)
}

func addCreate(s *server.MCPServer, svc *Service) {
	s.AddTool(
		mcp.NewTool("create_version",
			mcp.WithDescription("Create a new App Store version for an app."),
			mcp.WithString("appId", mcp.Required(), mcp.Description("App Store Connect app id.")),
			mcp.WithString("versionString", mcp.Required(), mcp.Description("Version string, e.g. 1.2.0.")),
			mcp.WithString("platform", mcp.Required(), mcp.Description("Platform: IOS, MAC_OS, TV_OS or VISION_OS.")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			appID, err := req.RequireString("appId")
			if err != nil {
				return toolkit.Fail(err)
			}
			versionString, err := req.RequireString("versionString")
			if err != nil {
				return toolkit.Fail(err)
			}
			platform, err := req.RequireString("platform")
			if err != nil {
				return toolkit.Fail(err)
			}
			out, err := svc.Create(ctx, CreateInput{AppID: appID, VersionString: versionString, Platform: platform})
			if err != nil {
				return toolkit.Fail(err)
			}
			return toolkit.Result(out)
		},
	)
}

func addUpdate(s *server.MCPServer, svc *Service) {
	s.AddTool(
		mcp.NewTool("update_version",
			mcp.WithDescription("Update a version's releaseType, earliestReleaseDate and/or copyright."),
			mcp.WithString("versionId", mcp.Required(), mcp.Description("appStoreVersion id.")),
			mcp.WithString("releaseType", mcp.Description("MANUAL, AFTER_APPROVAL or SCHEDULED.")),
			mcp.WithString("earliestReleaseDate", mcp.Description("ISO-8601 date for SCHEDULED releases.")),
			mcp.WithString("copyright", mcp.Description("Copyright string.")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			versionID, err := req.RequireString("versionId")
			if err != nil {
				return toolkit.Fail(err)
			}
			out, err := svc.Update(ctx, UpdateInput{
				VersionID:           versionID,
				ReleaseType:         req.GetString("releaseType", ""),
				EarliestReleaseDate: req.GetString("earliestReleaseDate", ""),
				Copyright:           req.GetString("copyright", ""),
			})
			if err != nil {
				return toolkit.Fail(err)
			}
			return toolkit.Result(out)
		},
	)
}
