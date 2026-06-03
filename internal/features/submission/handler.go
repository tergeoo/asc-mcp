package submission

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/tergrigoryantc/asc-mcp/internal/shared/toolkit"
)

// Register wires review submission tools onto the MCP server.
func Register(s *server.MCPServer, d *toolkit.Deps) {
	svc := NewService(d)
	addCreate(s, svc)
	addAddVersion(s, svc)
	addSubmit(s, svc)
	addStatus(s, svc)
}

func addCreate(s *server.MCPServer, svc *Service) {
	s.AddTool(
		mcp.NewTool("create_review_submission",
			mcp.WithDescription("Create a review submission for an app and platform."),
			mcp.WithString("appId", mcp.Required(), mcp.Description("App Store Connect app id.")),
			mcp.WithString("platform", mcp.Required(), mcp.Description("IOS, MAC_OS, TV_OS or VISION_OS.")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			appID, err := req.RequireString("appId")
			if err != nil {
				return toolkit.Fail(err)
			}
			platform, err := req.RequireString("platform")
			if err != nil {
				return toolkit.Fail(err)
			}
			out, err := svc.Create(ctx, appID, platform)
			if err != nil {
				return toolkit.Fail(err)
			}
			return toolkit.Result(out)
		},
	)
}

func addAddVersion(s *server.MCPServer, svc *Service) {
	s.AddTool(
		mcp.NewTool("add_version_to_submission",
			mcp.WithDescription("Attach an App Store version to a review submission."),
			mcp.WithString("submissionId", mcp.Required(), mcp.Description("reviewSubmission id.")),
			mcp.WithString("versionId", mcp.Required(), mcp.Description("appStoreVersion id.")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			submissionID, err := req.RequireString("submissionId")
			if err != nil {
				return toolkit.Fail(err)
			}
			versionID, err := req.RequireString("versionId")
			if err != nil {
				return toolkit.Fail(err)
			}
			out, err := svc.AddVersion(ctx, submissionID, versionID)
			if err != nil {
				return toolkit.Fail(err)
			}
			return toolkit.Result(out)
		},
	)
}

func addSubmit(s *server.MCPServer, svc *Service) {
	s.AddTool(
		mcp.NewTool("submit_for_review",
			mcp.WithDescription("Finalize and submit a review submission to App Review."),
			mcp.WithString("submissionId", mcp.Required(), mcp.Description("reviewSubmission id.")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			submissionID, err := req.RequireString("submissionId")
			if err != nil {
				return toolkit.Fail(err)
			}
			out, err := svc.Submit(ctx, submissionID)
			if err != nil {
				return toolkit.Fail(err)
			}
			return toolkit.Result(out)
		},
	)
}

func addStatus(s *server.MCPServer, svc *Service) {
	s.AddTool(
		mcp.NewTool("get_submission_status",
			mcp.WithDescription("Get the current state of a review submission."),
			mcp.WithString("submissionId", mcp.Required(), mcp.Description("reviewSubmission id.")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			submissionID, err := req.RequireString("submissionId")
			if err != nil {
				return toolkit.Fail(err)
			}
			out, err := svc.Status(ctx, submissionID)
			if err != nil {
				return toolkit.Fail(err)
			}
			return toolkit.Result(out)
		},
	)
}
