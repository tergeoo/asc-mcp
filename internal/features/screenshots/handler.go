package screenshots

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/tergrigoryantc/asc-mcp/internal/shared/toolkit"
)

// Register wires screenshot tools onto the MCP server.
func Register(s *server.MCPServer, d *toolkit.Deps) {
	svc := NewService(d)
	addList(s, svc)
	addUpload(s, svc)
	addDelete(s, svc)
	addProductReview(s, svc)
}

func addProductReview(s *server.MCPServer, svc *Service) {
	s.AddTool(
		mcp.NewTool("upload_product_review_screenshot",
			mcp.WithDescription("Upload the App Store review screenshot for an IAP or subscription "+
				"(reserve -> upload -> commit). Clears the missing-review-screenshot part of a product's "+
				"MISSING_METADATA. A representative paywall image is fine; it does not require live products."),
			mcp.WithString("productId", mcp.Required(), mcp.Description("inAppPurchase id or subscription id.")),
			mcp.WithString("productType", mcp.Required(), mcp.Description("'iap' or 'subscription'.")),
			mcp.WithString("filePath", mcp.Required(), mcp.Description("Absolute path to a .png/.jpg file on the server host.")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			productID, err := req.RequireString("productId")
			if err != nil {
				return toolkit.Fail(err)
			}
			productType, err := req.RequireString("productType")
			if err != nil {
				return toolkit.Fail(err)
			}
			filePath, err := req.RequireString("filePath")
			if err != nil {
				return toolkit.Fail(err)
			}
			out, err := svc.UploadProductReviewScreenshot(ctx, ProductReviewInput{
				ProductID: productID, ProductType: productType, FilePath: filePath,
			})
			if err != nil {
				return toolkit.Fail(err)
			}
			return toolkit.Result(out)
		},
	)
}

func addList(s *server.MCPServer, svc *Service) {
	s.AddTool(
		mcp.NewTool("list_screenshot_sets",
			mcp.WithDescription("List screenshot sets and their screenshots for a version localization."),
			mcp.WithString("versionLocalizationId", mcp.Required(), mcp.Description("appStoreVersionLocalization id.")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			id, err := req.RequireString("versionLocalizationId")
			if err != nil {
				return toolkit.Fail(err)
			}
			out, err := svc.ListSets(ctx, id)
			if err != nil {
				return toolkit.Fail(err)
			}
			return toolkit.Result(map[string]any{"screenshotSets": out})
		},
	)
}

func addUpload(s *server.MCPServer, svc *Service) {
	s.AddTool(
		mcp.NewTool("upload_screenshot",
			mcp.WithDescription("Upload a screenshot: reserves the asset, uploads it in chunks, and commits it "+
				"with an MD5 checksum. Creates the screenshot set for the display type if needed."),
			mcp.WithString("versionLocalizationId", mcp.Required(), mcp.Description("appStoreVersionLocalization id.")),
			mcp.WithString("displayType", mcp.Required(), mcp.Description("Screenshot display type, e.g. APP_IPHONE_67, APP_IPAD_PRO_3GEN_129.")),
			mcp.WithString("filePath", mcp.Required(), mcp.Description("Absolute path to a .png/.jpg file on the server host.")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			versionLocID, err := req.RequireString("versionLocalizationId")
			if err != nil {
				return toolkit.Fail(err)
			}
			displayType, err := req.RequireString("displayType")
			if err != nil {
				return toolkit.Fail(err)
			}
			filePath, err := req.RequireString("filePath")
			if err != nil {
				return toolkit.Fail(err)
			}
			out, err := svc.Upload(ctx, UploadInput{
				VersionLocalizationID: versionLocID, DisplayType: displayType, FilePath: filePath,
			})
			if err != nil {
				return toolkit.Fail(err)
			}
			return toolkit.Result(out)
		},
	)
}

func addDelete(s *server.MCPServer, svc *Service) {
	s.AddTool(
		mcp.NewTool("delete_screenshot",
			mcp.WithDescription("Delete a screenshot or an entire screenshot set."),
			mcp.WithString("id", mcp.Required(), mcp.Description("appScreenshot id, or appScreenshotSet id when kind=set.")),
			mcp.WithString("kind", mcp.Description("'screenshot' (default) or 'set'.")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			id, err := req.RequireString("id")
			if err != nil {
				return toolkit.Fail(err)
			}
			out, err := svc.Delete(ctx, DeleteInput{ID: id, Kind: req.GetString("kind", "")})
			if err != nil {
				return toolkit.Fail(err)
			}
			return toolkit.Result(out)
		},
	)
}
