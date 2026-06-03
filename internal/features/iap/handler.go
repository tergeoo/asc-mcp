package iap

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/tergeoo/asc-mcp/internal/shared/toolkit"
)

// Register wires IAP/subscription tools onto the MCP server.
func Register(s *server.MCPServer, d *toolkit.Deps) {
	svc := NewService(d)
	addList(s, svc)
	addCreate(s, svc)
	addUpdateIAPLoc(s, svc)
	addUpdateSubLoc(s, svc)
	addCreateSubGroup(s, svc)
	addSetGroupLoc(s, svc)
	addCreateSubscription(s, svc)
	addSetSubAvailability(s, svc)
	addSetSubPrice(s, svc)
	addSetIAPPrice(s, svc)
	addSetIAPAvailability(s, svc)
}

func addSetIAPAvailability(s *server.MCPServer, svc *Service) {
	s.AddTool(
		mcp.NewTool("set_iap_availability",
			mcp.WithDescription("Set the territories a one-time IAP is sold in (prerequisite for clearing MISSING_METADATA). "+
				"By default enables all App Store territories."),
			mcp.WithString("iapId", mcp.Required(), mcp.Description("inAppPurchase (v2) id.")),
			mcp.WithBoolean("allTerritories", mcp.Description("Enable all territories (default true).")),
			mcp.WithArray("territories", mcp.Description("Specific territory ids when allTerritories is false."),
				mcp.WithStringItems()),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			iapID, err := req.RequireString("iapId")
			if err != nil {
				return toolkit.Fail(err)
			}
			out, err := svc.SetIAPAvailability(ctx, iapID, req.GetBool("allTerritories", true), req.GetStringSlice("territories", nil))
			if err != nil {
				return toolkit.Fail(err)
			}
			return toolkit.Result(out)
		},
	)
}

func addSetSubAvailability(s *server.MCPServer, svc *Service) {
	s.AddTool(
		mcp.NewTool("set_subscription_availability",
			mcp.WithDescription("Set the territories a subscription is sold in (prerequisite for pricing). "+
				"By default enables all App Store territories."),
			mcp.WithString("subscriptionId", mcp.Required(), mcp.Description("subscription id.")),
			mcp.WithBoolean("allTerritories", mcp.Description("Enable all territories (default true).")),
			mcp.WithArray("territories", mcp.Description("Specific territory ids (ISO alpha-3) when allTerritories is false."),
				mcp.WithStringItems()),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			subID, err := req.RequireString("subscriptionId")
			if err != nil {
				return toolkit.Fail(err)
			}
			all := req.GetBool("allTerritories", true)
			out, err := svc.SetSubscriptionAvailability(ctx, subID, all, req.GetStringSlice("territories", nil))
			if err != nil {
				return toolkit.Fail(err)
			}
			return toolkit.Result(out)
		},
	)
}

func addCreateSubGroup(s *server.MCPServer, svc *Service) {
	s.AddTool(
		mcp.NewTool("create_subscription_group",
			mcp.WithDescription("Create a subscription group for an app (auto-renewable subscriptions live in a group)."),
			mcp.WithString("appId", mcp.Required(), mcp.Description("App Store Connect app id.")),
			mcp.WithString("referenceName", mcp.Required(), mcp.Description("Internal reference name for the group.")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			appID, err := req.RequireString("appId")
			if err != nil {
				return toolkit.Fail(err)
			}
			name, err := req.RequireString("referenceName")
			if err != nil {
				return toolkit.Fail(err)
			}
			out, err := svc.CreateSubscriptionGroup(ctx, appID, name)
			if err != nil {
				return toolkit.Fail(err)
			}
			return toolkit.Result(out)
		},
	)
}

func addSetGroupLoc(s *server.MCPServer, svc *Service) {
	s.AddTool(
		mcp.NewTool("set_subscription_group_localization",
			mcp.WithDescription("Set a subscription group's localized display name (required metadata)."),
			mcp.WithString("groupId", mcp.Required(), mcp.Description("subscriptionGroup id.")),
			mcp.WithString("locale", mcp.Required(), mcp.Description("Locale code, e.g. en-US.")),
			mcp.WithString("name", mcp.Required(), mcp.Description("Localized group display name.")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			groupID, err := req.RequireString("groupId")
			if err != nil {
				return toolkit.Fail(err)
			}
			locale, err := req.RequireString("locale")
			if err != nil {
				return toolkit.Fail(err)
			}
			name, err := req.RequireString("name")
			if err != nil {
				return toolkit.Fail(err)
			}
			out, err := svc.SetGroupLocalization(ctx, groupID, locale, name)
			if err != nil {
				return toolkit.Fail(err)
			}
			return toolkit.Result(out)
		},
	)
}

func addCreateSubscription(s *server.MCPServer, svc *Service) {
	s.AddTool(
		mcp.NewTool("create_subscription",
			mcp.WithDescription("Create an auto-renewable subscription in a subscription group."),
			mcp.WithString("groupId", mcp.Required(), mcp.Description("subscriptionGroup id.")),
			mcp.WithString("name", mcp.Required(), mcp.Description("Reference name (<=30 chars).")),
			mcp.WithString("productId", mcp.Required(), mcp.Description("Product id, e.g. app.fiftytwo.weekly.")),
			mcp.WithString("subscriptionPeriod", mcp.Required(), mcp.Description("ONE_WEEK, ONE_MONTH, TWO_MONTHS, THREE_MONTHS, SIX_MONTHS or ONE_YEAR.")),
			mcp.WithNumber("groupLevel", mcp.Description("Rank within the group (1 = top). Defaults to 1.")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			groupID, err := req.RequireString("groupId")
			if err != nil {
				return toolkit.Fail(err)
			}
			name, err := req.RequireString("name")
			if err != nil {
				return toolkit.Fail(err)
			}
			productID, err := req.RequireString("productId")
			if err != nil {
				return toolkit.Fail(err)
			}
			period, err := req.RequireString("subscriptionPeriod")
			if err != nil {
				return toolkit.Fail(err)
			}
			out, err := svc.CreateSubscription(ctx, CreateSubscriptionInput{
				GroupID: groupID, Name: name, ProductID: productID, Period: period,
				GroupLevel: int(req.GetFloat("groupLevel", 1)),
			})
			if err != nil {
				return toolkit.Fail(err)
			}
			return toolkit.Result(out)
		},
	)
}

func addSetSubPrice(s *server.MCPServer, svc *Service) {
	s.AddTool(
		mcp.NewTool("set_subscription_price",
			mcp.WithDescription("Set a subscription's base-territory price (App Store Connect equalizes other territories). "+
				"Resolves the target customer price to a price point."),
			mcp.WithString("subscriptionId", mcp.Required(), mcp.Description("subscription id.")),
			mcp.WithString("price", mcp.Required(), mcp.Description("Target customer price, e.g. 3.99.")),
			mcp.WithString("territory", mcp.Description("Base territory id (ISO alpha-3). Defaults to USA.")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			subID, err := req.RequireString("subscriptionId")
			if err != nil {
				return toolkit.Fail(err)
			}
			price, err := req.RequireString("price")
			if err != nil {
				return toolkit.Fail(err)
			}
			out, err := svc.SetSubscriptionPrice(ctx, subID, req.GetString("territory", ""), price)
			if err != nil {
				return toolkit.Fail(err)
			}
			return toolkit.Result(out)
		},
	)
}

func addSetIAPPrice(s *server.MCPServer, svc *Service) {
	s.AddTool(
		mcp.NewTool("set_iap_price",
			mcp.WithDescription("Set a one-time IAP's price via a price schedule (base territory equalized). "+
				"Resolves the target customer price to a price point."),
			mcp.WithString("iapId", mcp.Required(), mcp.Description("inAppPurchase (v2) id.")),
			mcp.WithString("price", mcp.Required(), mcp.Description("Target customer price, e.g. 79.99.")),
			mcp.WithString("territory", mcp.Description("Base territory id (ISO alpha-3). Defaults to USA.")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			iapID, err := req.RequireString("iapId")
			if err != nil {
				return toolkit.Fail(err)
			}
			price, err := req.RequireString("price")
			if err != nil {
				return toolkit.Fail(err)
			}
			out, err := svc.SetIAPPrice(ctx, iapID, req.GetString("territory", ""), price)
			if err != nil {
				return toolkit.Fail(err)
			}
			return toolkit.Result(out)
		},
	)
}

func addList(s *server.MCPServer, svc *Service) {
	s.AddTool(
		mcp.NewTool("list_iaps",
			mcp.WithDescription("List an app's in-app purchases (v2) and subscriptions."),
			mcp.WithString("appId", mcp.Required(), mcp.Description("App Store Connect app id.")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			appID, err := req.RequireString("appId")
			if err != nil {
				return toolkit.Fail(err)
			}
			out, err := svc.List(ctx, appID)
			if err != nil {
				return toolkit.Fail(err)
			}
			return toolkit.Result(out)
		},
	)
}

func addCreate(s *server.MCPServer, svc *Service) {
	s.AddTool(
		mcp.NewTool("create_iap",
			mcp.WithDescription("Create a consumable or non-consumable in-app purchase."),
			mcp.WithString("appId", mcp.Required(), mcp.Description("App Store Connect app id.")),
			mcp.WithString("name", mcp.Required(), mcp.Description("Reference name (<=30 chars).")),
			mcp.WithString("productId", mcp.Required(), mcp.Description("Product id, e.g. com.app.coins100.")),
			mcp.WithString("type", mcp.Required(), mcp.Description("CONSUMABLE, NON_CONSUMABLE or NON_RENEWING_SUBSCRIPTION.")),
			mcp.WithString("reviewNote", mcp.Description("Optional review note.")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			appID, err := req.RequireString("appId")
			if err != nil {
				return toolkit.Fail(err)
			}
			name, err := req.RequireString("name")
			if err != nil {
				return toolkit.Fail(err)
			}
			productID, err := req.RequireString("productId")
			if err != nil {
				return toolkit.Fail(err)
			}
			typ, err := req.RequireString("type")
			if err != nil {
				return toolkit.Fail(err)
			}
			out, err := svc.Create(ctx, CreateInput{
				AppID: appID, Name: name, ProductID: productID, Type: typ,
				ReviewNote: req.GetString("reviewNote", ""),
			})
			if err != nil {
				return toolkit.Fail(err)
			}
			return toolkit.Result(out)
		},
	)
}

func addUpdateIAPLoc(s *server.MCPServer, svc *Service) {
	s.AddTool(
		mcp.NewTool("update_iap_localization",
			mcp.WithDescription("Create or update an IAP localization (name, description) for a locale."),
			mcp.WithString("iapId", mcp.Required(), mcp.Description("inAppPurchase (v2) id.")),
			mcp.WithString("locale", mcp.Required(), mcp.Description("Locale code, e.g. en-US.")),
			mcp.WithString("name", mcp.Description("Localized display name (<=30 chars).")),
			mcp.WithString("description", mcp.Description("Localized description (<=45 chars).")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			iapID, err := req.RequireString("iapId")
			if err != nil {
				return toolkit.Fail(err)
			}
			locale, err := req.RequireString("locale")
			if err != nil {
				return toolkit.Fail(err)
			}
			out, err := svc.UpdateIAPLocalization(ctx, UpdateIAPLocalizationInput{
				IAPID: iapID, Locale: locale,
				Fields: LocFields{Name: req.GetString("name", ""), Description: req.GetString("description", "")},
			})
			if err != nil {
				return toolkit.Fail(err)
			}
			return toolkit.Result(out)
		},
	)
}

func addUpdateSubLoc(s *server.MCPServer, svc *Service) {
	s.AddTool(
		mcp.NewTool("update_subscription_localization",
			mcp.WithDescription("Create or update a subscription localization (name, description) for a locale."),
			mcp.WithString("subscriptionId", mcp.Required(), mcp.Description("subscription id.")),
			mcp.WithString("locale", mcp.Required(), mcp.Description("Locale code, e.g. en-US.")),
			mcp.WithString("name", mcp.Description("Localized display name.")),
			mcp.WithString("description", mcp.Description("Localized description.")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			subID, err := req.RequireString("subscriptionId")
			if err != nil {
				return toolkit.Fail(err)
			}
			locale, err := req.RequireString("locale")
			if err != nil {
				return toolkit.Fail(err)
			}
			out, err := svc.UpdateSubscriptionLocalization(ctx, UpdateSubscriptionLocalizationInput{
				SubscriptionID: subID, Locale: locale,
				Fields: LocFields{Name: req.GetString("name", ""), Description: req.GetString("description", "")},
			})
			if err != nil {
				return toolkit.Fail(err)
			}
			return toolkit.Result(out)
		},
	)
}
