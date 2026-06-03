package apps

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/tergrigoryantc/asc-mcp/internal/shared/toolkit"
)

// Register wires the apps discovery tools onto the MCP server.
func Register(s *server.MCPServer, d *toolkit.Deps) {
	svc := NewService(d)

	s.AddTool(
		mcp.NewTool("list_apps",
			mcp.WithDescription("List all apps visible to the configured App Store Connect credentials (id, bundleId, name)."),
		),
		func(ctx context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			apps, err := svc.ListApps(ctx)
			if err != nil {
				return toolkit.Fail(err)
			}
			return toolkit.Result(map[string]any{"apps": apps})
		},
	)

	s.AddTool(
		mcp.NewTool("get_app_info",
			mcp.WithDescription("Get an app's appInfo and its current appInfo localizations (name, subtitle, privacyPolicyUrl)."),
			mcp.WithString("appId", mcp.Required(), mcp.Description("App Store Connect app id.")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			appID, err := req.RequireString("appId")
			if err != nil {
				return toolkit.Fail(err)
			}
			info, err := svc.GetAppInfo(ctx, appID)
			if err != nil {
				return toolkit.Fail(err)
			}
			return toolkit.Result(info)
		},
	)

	s.AddTool(
		mcp.NewTool("set_app_availability",
			mcp.WithDescription("Set the territories an app is available in. By default enables all App Store territories."),
			mcp.WithString("appId", mcp.Required(), mcp.Description("App Store Connect app id.")),
			mcp.WithBoolean("allTerritories", mcp.Description("Enable all territories (default true).")),
			mcp.WithArray("territories", mcp.Description("Specific territory ids (ISO alpha-3) when allTerritories is false."),
				mcp.WithStringItems()),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			appID, err := req.RequireString("appId")
			if err != nil {
				return toolkit.Fail(err)
			}
			out, err := svc.SetAvailability(ctx, appID, req.GetBool("allTerritories", true), req.GetStringSlice("territories", nil))
			if err != nil {
				return toolkit.Fail(err)
			}
			return toolkit.Result(out)
		},
	)

	s.AddTool(
		mcp.NewTool("set_app_price",
			mcp.WithDescription("Set an app's price (base territory equalized across the rest). Use price \"0\" to make the app free."),
			mcp.WithString("appId", mcp.Required(), mcp.Description("App Store Connect app id.")),
			mcp.WithString("price", mcp.Required(), mcp.Description("Target customer price, e.g. 0 (free) or 4.99.")),
			mcp.WithString("territory", mcp.Description("Base territory id (ISO alpha-3). Defaults to USA.")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			appID, err := req.RequireString("appId")
			if err != nil {
				return toolkit.Fail(err)
			}
			price, err := req.RequireString("price")
			if err != nil {
				return toolkit.Fail(err)
			}
			out, err := svc.SetPrice(ctx, appID, req.GetString("territory", ""), price)
			if err != nil {
				return toolkit.Fail(err)
			}
			return toolkit.Result(out)
		},
	)
}
