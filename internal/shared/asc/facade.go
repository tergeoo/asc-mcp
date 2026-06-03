package asc

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/tergeoo/asc-mcp/internal/shared/mcperr"
)

// TokenFunc returns a valid ASC bearer token. Implemented by auth.Provider.
type TokenFunc func() (string, error)

// Resource is a generic JSON:API resource object. The service layer reads the
// fields it needs out of Attributes/Relationships rather than depending on the
// large generated attribute structs.
type Resource struct {
	Type          string          `json:"type"`
	ID            string          `json:"id"`
	Attributes    json.RawMessage `json:"attributes,omitempty"`
	Relationships json.RawMessage `json:"relationships,omitempty"`
}

// Attr unmarshals the resource attributes into out.
func (r Resource) Attr(out any) error {
	if len(r.Attributes) == 0 {
		return nil
	}
	return json.Unmarshal(r.Attributes, out)
}

type singleDoc struct {
	Data     Resource        `json:"data"`
	Included []Resource      `json:"included,omitempty"`
	Links    json.RawMessage `json:"links,omitempty"`
}

type collectionDoc struct {
	Data     []Resource      `json:"data"`
	Included []Resource      `json:"included,omitempty"`
	Links    json.RawMessage `json:"links,omitempty"`
}

// Facade wraps the generated client with higher-level, JSON:API-aware methods.
// All write payloads are constructed as JSON:API maps and routed through the
// generated *WithBody methods, so requests still flow through generated code.
type Facade struct {
	c       *ClientWithResponses
	baseURL string
	token   TokenFunc
	http    *http.Client
}

// NewFacade builds a facade. The token function is invoked per request via a
// generated RequestEditorFn that injects the Authorization header.
func NewFacade(baseURL string, token TokenFunc) (*Facade, error) {
	httpClient := &http.Client{Timeout: 60 * time.Second}
	editor := func(ctx context.Context, req *http.Request) error {
		tok, err := token()
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+tok)
		return nil
	}
	c, err := NewClientWithResponses(
		baseURL,
		WithHTTPClient(httpClient),
		WithRequestEditorFn(editor),
	)
	if err != nil {
		return nil, err
	}
	return &Facade{c: c, baseURL: baseURL, token: token, http: httpClient}, nil
}

const contentTypeJSON = "application/json"

func body(v any) (io.Reader, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	return bytes.NewReader(b), nil
}

// parseOne validates the HTTP status and decodes a single-resource document.
func parseOne(op string, status int, hdr http.Header, raw []byte) (Resource, error) {
	if e := mcperr.FromResponse(status, hdr.Get("Retry-After"), raw); e != nil {
		return Resource{}, e
	}
	var doc singleDoc
	if err := json.Unmarshal(raw, &doc); err != nil {
		return Resource{}, mcperr.Wrap(op+": decode", err)
	}
	return doc.Data, nil
}

// parseMany validates the HTTP status and decodes a collection document.
func parseMany(op string, status int, hdr http.Header, raw []byte) ([]Resource, error) {
	if e := mcperr.FromResponse(status, hdr.Get("Retry-After"), raw); e != nil {
		return nil, e
	}
	var doc collectionDoc
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, mcperr.Wrap(op+": decode", err)
	}
	return doc.Data, nil
}

// parseStatus validates a no-body response (e.g. DELETE).
func parseStatus(op string, status int, hdr http.Header, raw []byte) error {
	if e := mcperr.FromResponse(status, hdr.Get("Retry-After"), raw); e != nil {
		return e
	}
	return nil
}

// ---- Apps -----------------------------------------------------------------

func (f *Facade) ListApps(ctx context.Context) ([]Resource, error) {
	resp, err := f.c.AppsGetCollectionWithResponse(ctx, &AppsGetCollectionParams{})
	if err != nil {
		return nil, mcperr.Wrap("list_apps", err)
	}
	return parseMany("list_apps", resp.StatusCode(), resp.HTTPResponse.Header, resp.Body)
}

func (f *Facade) GetApp(ctx context.Context, id string) (Resource, error) {
	resp, err := f.c.AppsGetInstanceWithResponse(ctx, id, &AppsGetInstanceParams{})
	if err != nil {
		return Resource{}, mcperr.Wrap("get_app", err)
	}
	return parseOne("get_app", resp.StatusCode(), resp.HTTPResponse.Header, resp.Body)
}

func (f *Facade) GetAppInfos(ctx context.Context, appID string) ([]Resource, error) {
	resp, err := f.c.AppsAppInfosGetToManyRelatedWithResponse(ctx, appID, &AppsAppInfosGetToManyRelatedParams{})
	if err != nil {
		return nil, mcperr.Wrap("get_app_infos", err)
	}
	return parseMany("get_app_infos", resp.StatusCode(), resp.HTTPResponse.Header, resp.Body)
}

func (f *Facade) GetAppInfoLocalizations(ctx context.Context, appInfoID string) ([]Resource, error) {
	resp, err := f.c.AppInfosAppInfoLocalizationsGetToManyRelatedWithResponse(ctx, appInfoID, &AppInfosAppInfoLocalizationsGetToManyRelatedParams{})
	if err != nil {
		return nil, mcperr.Wrap("get_app_info_localizations", err)
	}
	return parseMany("get_app_info_localizations", resp.StatusCode(), resp.HTTPResponse.Header, resp.Body)
}

// ---- App info localizations ----------------------------------------------

func (f *Facade) CreateAppInfoLocalization(ctx context.Context, payload any) (Resource, error) {
	rdr, err := body(payload)
	if err != nil {
		return Resource{}, mcperr.Wrap("create_app_info_localization", err)
	}
	resp, err := f.c.AppInfoLocalizationsCreateInstanceWithBodyWithResponse(ctx, contentTypeJSON, rdr)
	if err != nil {
		return Resource{}, mcperr.Wrap("create_app_info_localization", err)
	}
	return parseOne("create_app_info_localization", resp.StatusCode(), resp.HTTPResponse.Header, resp.Body)
}

func (f *Facade) UpdateAppInfoLocalization(ctx context.Context, id string, payload any) (Resource, error) {
	rdr, err := body(payload)
	if err != nil {
		return Resource{}, mcperr.Wrap("update_app_info_localization", err)
	}
	resp, err := f.c.AppInfoLocalizationsUpdateInstanceWithBodyWithResponse(ctx, id, contentTypeJSON, rdr)
	if err != nil {
		return Resource{}, mcperr.Wrap("update_app_info_localization", err)
	}
	return parseOne("update_app_info_localization", resp.StatusCode(), resp.HTTPResponse.Header, resp.Body)
}

// ---- Versions -------------------------------------------------------------

func (f *Facade) GetAppVersions(ctx context.Context, appID string) ([]Resource, error) {
	resp, err := f.c.AppsAppStoreVersionsGetToManyRelatedWithResponse(ctx, appID, &AppsAppStoreVersionsGetToManyRelatedParams{})
	if err != nil {
		return nil, mcperr.Wrap("list_versions", err)
	}
	return parseMany("list_versions", resp.StatusCode(), resp.HTTPResponse.Header, resp.Body)
}

func (f *Facade) GetVersion(ctx context.Context, id string) (Resource, error) {
	resp, err := f.c.AppStoreVersionsGetInstanceWithResponse(ctx, id, &AppStoreVersionsGetInstanceParams{})
	if err != nil {
		return Resource{}, mcperr.Wrap("get_version", err)
	}
	return parseOne("get_version", resp.StatusCode(), resp.HTTPResponse.Header, resp.Body)
}

func (f *Facade) CreateVersion(ctx context.Context, payload any) (Resource, error) {
	rdr, err := body(payload)
	if err != nil {
		return Resource{}, mcperr.Wrap("create_version", err)
	}
	resp, err := f.c.AppStoreVersionsCreateInstanceWithBodyWithResponse(ctx, contentTypeJSON, rdr)
	if err != nil {
		return Resource{}, mcperr.Wrap("create_version", err)
	}
	return parseOne("create_version", resp.StatusCode(), resp.HTTPResponse.Header, resp.Body)
}

func (f *Facade) UpdateVersion(ctx context.Context, id string, payload any) (Resource, error) {
	rdr, err := body(payload)
	if err != nil {
		return Resource{}, mcperr.Wrap("update_version", err)
	}
	resp, err := f.c.AppStoreVersionsUpdateInstanceWithBodyWithResponse(ctx, id, contentTypeJSON, rdr)
	if err != nil {
		return Resource{}, mcperr.Wrap("update_version", err)
	}
	return parseOne("update_version", resp.StatusCode(), resp.HTTPResponse.Header, resp.Body)
}

// ---- Version localizations ------------------------------------------------

func (f *Facade) GetVersionLocalizations(ctx context.Context, versionID string) ([]Resource, error) {
	resp, err := f.c.AppStoreVersionsAppStoreVersionLocalizationsGetToManyRelatedWithResponse(ctx, versionID, &AppStoreVersionsAppStoreVersionLocalizationsGetToManyRelatedParams{})
	if err != nil {
		return nil, mcperr.Wrap("get_version_localizations", err)
	}
	return parseMany("get_version_localizations", resp.StatusCode(), resp.HTTPResponse.Header, resp.Body)
}

func (f *Facade) CreateVersionLocalization(ctx context.Context, payload any) (Resource, error) {
	rdr, err := body(payload)
	if err != nil {
		return Resource{}, mcperr.Wrap("create_version_localization", err)
	}
	resp, err := f.c.AppStoreVersionLocalizationsCreateInstanceWithBodyWithResponse(ctx, contentTypeJSON, rdr)
	if err != nil {
		return Resource{}, mcperr.Wrap("create_version_localization", err)
	}
	return parseOne("create_version_localization", resp.StatusCode(), resp.HTTPResponse.Header, resp.Body)
}

func (f *Facade) UpdateVersionLocalization(ctx context.Context, id string, payload any) (Resource, error) {
	rdr, err := body(payload)
	if err != nil {
		return Resource{}, mcperr.Wrap("update_version_localization", err)
	}
	resp, err := f.c.AppStoreVersionLocalizationsUpdateInstanceWithBodyWithResponse(ctx, id, contentTypeJSON, rdr)
	if err != nil {
		return Resource{}, mcperr.Wrap("update_version_localization", err)
	}
	return parseOne("update_version_localization", resp.StatusCode(), resp.HTTPResponse.Header, resp.Body)
}

// ---- IAP / subscriptions --------------------------------------------------

func (f *Facade) GetAppIAPs(ctx context.Context, appID string) ([]Resource, error) {
	resp, err := f.c.AppsInAppPurchasesV2GetToManyRelatedWithResponse(ctx, appID, &AppsInAppPurchasesV2GetToManyRelatedParams{})
	if err != nil {
		return nil, mcperr.Wrap("list_iaps", err)
	}
	return parseMany("list_iaps", resp.StatusCode(), resp.HTTPResponse.Header, resp.Body)
}

func (f *Facade) CreateIAP(ctx context.Context, payload any) (Resource, error) {
	rdr, err := body(payload)
	if err != nil {
		return Resource{}, mcperr.Wrap("create_iap", err)
	}
	resp, err := f.c.InAppPurchasesV2CreateInstanceWithBodyWithResponse(ctx, contentTypeJSON, rdr)
	if err != nil {
		return Resource{}, mcperr.Wrap("create_iap", err)
	}
	return parseOne("create_iap", resp.StatusCode(), resp.HTTPResponse.Header, resp.Body)
}

// CreateIAPAvailability sets the territories a one-time IAP is sold in
// (required to clear MISSING_METADATA, like subscriptions).
func (f *Facade) CreateIAPAvailability(ctx context.Context, payload any) (Resource, error) {
	rdr, err := body(payload)
	if err != nil {
		return Resource{}, mcperr.Wrap("set_iap_availability", err)
	}
	resp, err := f.c.InAppPurchaseAvailabilitiesCreateInstanceWithBodyWithResponse(ctx, contentTypeJSON, rdr)
	if err != nil {
		return Resource{}, mcperr.Wrap("set_iap_availability", err)
	}
	return parseOne("set_iap_availability", resp.StatusCode(), resp.HTTPResponse.Header, resp.Body)
}

func (f *Facade) GetIAPLocalizations(ctx context.Context, iapID string) ([]Resource, error) {
	resp, err := f.c.InAppPurchasesV2InAppPurchaseLocalizationsGetToManyRelatedWithResponse(ctx, iapID, &InAppPurchasesV2InAppPurchaseLocalizationsGetToManyRelatedParams{})
	if err != nil {
		return nil, mcperr.Wrap("get_iap_localizations", err)
	}
	return parseMany("get_iap_localizations", resp.StatusCode(), resp.HTTPResponse.Header, resp.Body)
}

func (f *Facade) CreateIAPLocalization(ctx context.Context, payload any) (Resource, error) {
	rdr, err := body(payload)
	if err != nil {
		return Resource{}, mcperr.Wrap("create_iap_localization", err)
	}
	resp, err := f.c.InAppPurchaseLocalizationsCreateInstanceWithBodyWithResponse(ctx, contentTypeJSON, rdr)
	if err != nil {
		return Resource{}, mcperr.Wrap("create_iap_localization", err)
	}
	return parseOne("create_iap_localization", resp.StatusCode(), resp.HTTPResponse.Header, resp.Body)
}

func (f *Facade) UpdateIAPLocalization(ctx context.Context, id string, payload any) (Resource, error) {
	rdr, err := body(payload)
	if err != nil {
		return Resource{}, mcperr.Wrap("update_iap_localization", err)
	}
	resp, err := f.c.InAppPurchaseLocalizationsUpdateInstanceWithBodyWithResponse(ctx, id, contentTypeJSON, rdr)
	if err != nil {
		return Resource{}, mcperr.Wrap("update_iap_localization", err)
	}
	return parseOne("update_iap_localization", resp.StatusCode(), resp.HTTPResponse.Header, resp.Body)
}

func (f *Facade) GetAppSubscriptionGroups(ctx context.Context, appID string) ([]Resource, error) {
	resp, err := f.c.AppsSubscriptionGroupsGetToManyRelatedWithResponse(ctx, appID, &AppsSubscriptionGroupsGetToManyRelatedParams{})
	if err != nil {
		return nil, mcperr.Wrap("list_subscription_groups", err)
	}
	return parseMany("list_subscription_groups", resp.StatusCode(), resp.HTTPResponse.Header, resp.Body)
}

func (f *Facade) GetGroupSubscriptions(ctx context.Context, groupID string) ([]Resource, error) {
	resp, err := f.c.SubscriptionGroupsSubscriptionsGetToManyRelatedWithResponse(ctx, groupID, &SubscriptionGroupsSubscriptionsGetToManyRelatedParams{})
	if err != nil {
		return nil, mcperr.Wrap("list_subscriptions", err)
	}
	return parseMany("list_subscriptions", resp.StatusCode(), resp.HTTPResponse.Header, resp.Body)
}

func (f *Facade) GetSubscriptionLocalizations(ctx context.Context, subscriptionID string) ([]Resource, error) {
	resp, err := f.c.SubscriptionsSubscriptionLocalizationsGetToManyRelatedWithResponse(ctx, subscriptionID, &SubscriptionsSubscriptionLocalizationsGetToManyRelatedParams{})
	if err != nil {
		return nil, mcperr.Wrap("get_subscription_localizations", err)
	}
	return parseMany("get_subscription_localizations", resp.StatusCode(), resp.HTTPResponse.Header, resp.Body)
}

func (f *Facade) CreateSubscriptionLocalization(ctx context.Context, payload any) (Resource, error) {
	rdr, err := body(payload)
	if err != nil {
		return Resource{}, mcperr.Wrap("create_subscription_localization", err)
	}
	resp, err := f.c.SubscriptionLocalizationsCreateInstanceWithBodyWithResponse(ctx, contentTypeJSON, rdr)
	if err != nil {
		return Resource{}, mcperr.Wrap("create_subscription_localization", err)
	}
	return parseOne("create_subscription_localization", resp.StatusCode(), resp.HTTPResponse.Header, resp.Body)
}

func (f *Facade) UpdateSubscriptionLocalization(ctx context.Context, id string, payload any) (Resource, error) {
	rdr, err := body(payload)
	if err != nil {
		return Resource{}, mcperr.Wrap("update_subscription_localization", err)
	}
	resp, err := f.c.SubscriptionLocalizationsUpdateInstanceWithBodyWithResponse(ctx, id, contentTypeJSON, rdr)
	if err != nil {
		return Resource{}, mcperr.Wrap("update_subscription_localization", err)
	}
	return parseOne("update_subscription_localization", resp.StatusCode(), resp.HTTPResponse.Header, resp.Body)
}

// ---- App-level availability & price ---------------------------------------

// GetAppPricePoints lists ALL app price points in a territory (paginated).
func (f *Facade) GetAppPricePoints(ctx context.Context, appID, territory string) ([]Resource, error) {
	u := fmt.Sprintf("%s/v1/apps/%s/appPricePoints?filter[territory]=%s&limit=200",
		f.baseURL, url.PathEscape(appID), url.QueryEscape(territory))
	return f.getAllPaged(ctx, "list_app_price_points", u)
}

// CreateAppAvailability sets the territories an app is available in.
func (f *Facade) CreateAppAvailability(ctx context.Context, payload any) (Resource, error) {
	rdr, err := body(payload)
	if err != nil {
		return Resource{}, mcperr.Wrap("set_app_availability", err)
	}
	resp, err := f.c.AppAvailabilitiesV2CreateInstanceWithBodyWithResponse(ctx, contentTypeJSON, rdr)
	if err != nil {
		return Resource{}, mcperr.Wrap("set_app_availability", err)
	}
	return parseOne("set_app_availability", resp.StatusCode(), resp.HTTPResponse.Header, resp.Body)
}

// CreateAppPriceSchedule sets an app's price (base territory equalized).
func (f *Facade) CreateAppPriceSchedule(ctx context.Context, payload any) (Resource, error) {
	rdr, err := body(payload)
	if err != nil {
		return Resource{}, mcperr.Wrap("set_app_price", err)
	}
	resp, err := f.c.AppPriceSchedulesCreateInstanceWithBodyWithResponse(ctx, contentTypeJSON, rdr)
	if err != nil {
		return Resource{}, mcperr.Wrap("set_app_price", err)
	}
	return parseOne("set_app_price", resp.StatusCode(), resp.HTTPResponse.Header, resp.Body)
}

// ---- Subscriptions & prices (creation) ------------------------------------

func (f *Facade) CreateSubscriptionGroup(ctx context.Context, payload any) (Resource, error) {
	rdr, err := body(payload)
	if err != nil {
		return Resource{}, mcperr.Wrap("create_subscription_group", err)
	}
	resp, err := f.c.SubscriptionGroupsCreateInstanceWithBodyWithResponse(ctx, contentTypeJSON, rdr)
	if err != nil {
		return Resource{}, mcperr.Wrap("create_subscription_group", err)
	}
	return parseOne("create_subscription_group", resp.StatusCode(), resp.HTTPResponse.Header, resp.Body)
}

func (f *Facade) CreateSubscriptionGroupLocalization(ctx context.Context, payload any) (Resource, error) {
	rdr, err := body(payload)
	if err != nil {
		return Resource{}, mcperr.Wrap("create_subscription_group_localization", err)
	}
	resp, err := f.c.SubscriptionGroupLocalizationsCreateInstanceWithBodyWithResponse(ctx, contentTypeJSON, rdr)
	if err != nil {
		return Resource{}, mcperr.Wrap("create_subscription_group_localization", err)
	}
	return parseOne("create_subscription_group_localization", resp.StatusCode(), resp.HTTPResponse.Header, resp.Body)
}

func (f *Facade) CreateSubscription(ctx context.Context, payload any) (Resource, error) {
	rdr, err := body(payload)
	if err != nil {
		return Resource{}, mcperr.Wrap("create_subscription", err)
	}
	resp, err := f.c.SubscriptionsCreateInstanceWithBodyWithResponse(ctx, contentTypeJSON, rdr)
	if err != nil {
		return Resource{}, mcperr.Wrap("create_subscription", err)
	}
	return parseOne("create_subscription", resp.StatusCode(), resp.HTTPResponse.Header, resp.Body)
}

// GetAllTerritories returns every App Store territory (paginated).
func (f *Facade) GetAllTerritories(ctx context.Context) ([]Resource, error) {
	u := fmt.Sprintf("%s/v1/territories?limit=200", f.baseURL)
	return f.getAllPaged(ctx, "list_territories", u)
}

// CreateSubscriptionAvailability sets the territories a subscription is sold in.
// Required before a subscription price can be created.
func (f *Facade) CreateSubscriptionAvailability(ctx context.Context, payload any) (Resource, error) {
	rdr, err := body(payload)
	if err != nil {
		return Resource{}, mcperr.Wrap("set_subscription_availability", err)
	}
	resp, err := f.c.SubscriptionAvailabilitiesCreateInstanceWithBodyWithResponse(ctx, contentTypeJSON, rdr)
	if err != nil {
		return Resource{}, mcperr.Wrap("set_subscription_availability", err)
	}
	return parseOne("set_subscription_availability", resp.StatusCode(), resp.HTTPResponse.Header, resp.Body)
}

func (f *Facade) CreateSubscriptionPrice(ctx context.Context, payload any) (Resource, error) {
	rdr, err := body(payload)
	if err != nil {
		return Resource{}, mcperr.Wrap("set_subscription_price", err)
	}
	resp, err := f.c.SubscriptionPricesCreateInstanceWithBodyWithResponse(ctx, contentTypeJSON, rdr)
	if err != nil {
		return Resource{}, mcperr.Wrap("set_subscription_price", err)
	}
	return parseOne("set_subscription_price", resp.StatusCode(), resp.HTTPResponse.Header, resp.Body)
}

// GetSubscriptionPricePoints lists ALL price points for a subscription in a
// single territory (paginated), used to resolve a target customer price to a
// price-point id. A territory can have several hundred price tiers, so a single
// page is not enough.
func (f *Facade) GetSubscriptionPricePoints(ctx context.Context, subID, territory string) ([]Resource, error) {
	u := fmt.Sprintf("%s/v1/subscriptions/%s/pricePoints?filter[territory]=%s&limit=200",
		f.baseURL, url.PathEscape(subID), url.QueryEscape(territory))
	return f.getAllPaged(ctx, "list_subscription_price_points", u)
}

// GetSubscriptionPricePointEqualizations returns the equivalent price points in
// every other territory for a given base price point (paginated). Used to price
// a subscription across all territories at the same tier.
func (f *Facade) GetSubscriptionPricePointEqualizations(ctx context.Context, pointID string) ([]Resource, error) {
	u := fmt.Sprintf("%s/v1/subscriptionPricePoints/%s/equalizations?limit=200",
		f.baseURL, url.PathEscape(pointID))
	return f.getAllPaged(ctx, "list_price_point_equalizations", u)
}

func (f *Facade) CreateIAPPriceSchedule(ctx context.Context, payload any) (Resource, error) {
	rdr, err := body(payload)
	if err != nil {
		return Resource{}, mcperr.Wrap("set_iap_price", err)
	}
	resp, err := f.c.InAppPurchasePriceSchedulesCreateInstanceWithBodyWithResponse(ctx, contentTypeJSON, rdr)
	if err != nil {
		return Resource{}, mcperr.Wrap("set_iap_price", err)
	}
	return parseOne("set_iap_price", resp.StatusCode(), resp.HTTPResponse.Header, resp.Body)
}

// GetIAPPricePoints lists ALL price points for an IAP in a single territory
// (paginated).
func (f *Facade) GetIAPPricePoints(ctx context.Context, iapID, territory string) ([]Resource, error) {
	u := fmt.Sprintf("%s/v2/inAppPurchases/%s/pricePoints?filter[territory]=%s&limit=200",
		f.baseURL, url.PathEscape(iapID), url.QueryEscape(territory))
	return f.getAllPaged(ctx, "list_iap_price_points", u)
}

// getAllPaged performs an authenticated GET and follows JSON:API links.next,
// accumulating every resource across pages.
func (f *Facade) getAllPaged(ctx context.Context, op, startURL string) ([]Resource, error) {
	var all []Resource
	next := startURL
	for next != "" {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, next, nil)
		if err != nil {
			return nil, mcperr.Wrap(op, err)
		}
		tok, err := f.token()
		if err != nil {
			return nil, mcperr.Wrap(op, err)
		}
		req.Header.Set("Authorization", "Bearer "+tok)
		resp, err := f.http.Do(req)
		if err != nil {
			return nil, mcperr.Wrap(op, err)
		}
		raw, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if e := mcperr.FromResponse(resp.StatusCode, resp.Header.Get("Retry-After"), raw); e != nil {
			return nil, e
		}
		var doc struct {
			Data  []Resource `json:"data"`
			Links struct {
				Next string `json:"next"`
			} `json:"links"`
		}
		if err := json.Unmarshal(raw, &doc); err != nil {
			return nil, mcperr.Wrap(op+": decode", err)
		}
		all = append(all, doc.Data...)
		next = doc.Links.Next
	}
	return all, nil
}

// ---- Screenshots ----------------------------------------------------------

func (f *Facade) GetVersionLocScreenshotSets(ctx context.Context, versionLocID string) ([]Resource, error) {
	resp, err := f.c.AppStoreVersionLocalizationsAppScreenshotSetsGetToManyRelatedWithResponse(ctx, versionLocID, &AppStoreVersionLocalizationsAppScreenshotSetsGetToManyRelatedParams{})
	if err != nil {
		return nil, mcperr.Wrap("list_screenshot_sets", err)
	}
	return parseMany("list_screenshot_sets", resp.StatusCode(), resp.HTTPResponse.Header, resp.Body)
}

func (f *Facade) CreateScreenshotSet(ctx context.Context, payload any) (Resource, error) {
	rdr, err := body(payload)
	if err != nil {
		return Resource{}, mcperr.Wrap("create_screenshot_set", err)
	}
	resp, err := f.c.AppScreenshotSetsCreateInstanceWithBodyWithResponse(ctx, contentTypeJSON, rdr)
	if err != nil {
		return Resource{}, mcperr.Wrap("create_screenshot_set", err)
	}
	return parseOne("create_screenshot_set", resp.StatusCode(), resp.HTTPResponse.Header, resp.Body)
}

func (f *Facade) DeleteScreenshotSet(ctx context.Context, id string) error {
	resp, err := f.c.AppScreenshotSetsDeleteInstanceWithResponse(ctx, id)
	if err != nil {
		return mcperr.Wrap("delete_screenshot_set", err)
	}
	return parseStatus("delete_screenshot_set", resp.StatusCode(), resp.HTTPResponse.Header, resp.Body)
}

func (f *Facade) GetSetScreenshots(ctx context.Context, setID string) ([]Resource, error) {
	resp, err := f.c.AppScreenshotSetsAppScreenshotsGetToManyRelatedWithResponse(ctx, setID, &AppScreenshotSetsAppScreenshotsGetToManyRelatedParams{})
	if err != nil {
		return nil, mcperr.Wrap("list_screenshots", err)
	}
	return parseMany("list_screenshots", resp.StatusCode(), resp.HTTPResponse.Header, resp.Body)
}

func (f *Facade) ReserveScreenshot(ctx context.Context, payload any) (Resource, error) {
	rdr, err := body(payload)
	if err != nil {
		return Resource{}, mcperr.Wrap("reserve_screenshot", err)
	}
	resp, err := f.c.AppScreenshotsCreateInstanceWithBodyWithResponse(ctx, contentTypeJSON, rdr)
	if err != nil {
		return Resource{}, mcperr.Wrap("reserve_screenshot", err)
	}
	return parseOne("reserve_screenshot", resp.StatusCode(), resp.HTTPResponse.Header, resp.Body)
}

func (f *Facade) CommitScreenshot(ctx context.Context, id string, payload any) (Resource, error) {
	rdr, err := body(payload)
	if err != nil {
		return Resource{}, mcperr.Wrap("commit_screenshot", err)
	}
	resp, err := f.c.AppScreenshotsUpdateInstanceWithBodyWithResponse(ctx, id, contentTypeJSON, rdr)
	if err != nil {
		return Resource{}, mcperr.Wrap("commit_screenshot", err)
	}
	return parseOne("commit_screenshot", resp.StatusCode(), resp.HTTPResponse.Header, resp.Body)
}

func (f *Facade) GetScreenshot(ctx context.Context, id string) (Resource, error) {
	resp, err := f.c.AppScreenshotsGetInstanceWithResponse(ctx, id, &AppScreenshotsGetInstanceParams{})
	if err != nil {
		return Resource{}, mcperr.Wrap("get_screenshot", err)
	}
	return parseOne("get_screenshot", resp.StatusCode(), resp.HTTPResponse.Header, resp.Body)
}

func (f *Facade) DeleteScreenshot(ctx context.Context, id string) error {
	resp, err := f.c.AppScreenshotsDeleteInstanceWithResponse(ctx, id)
	if err != nil {
		return mcperr.Wrap("delete_screenshot", err)
	}
	return parseStatus("delete_screenshot", resp.StatusCode(), resp.HTTPResponse.Header, resp.Body)
}

// UploadChunk PUTs/POSTs a single binary chunk to an upload operation URL
// returned by ASC when reserving an asset. These URLs are pre-signed and are
// not part of the generated client, so we issue them directly. The bearer
// token is NOT attached (the URL is already authorized).
func (f *Facade) UploadChunk(ctx context.Context, method, url string, headers map[string]string, data []byte) error {
	req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewReader(data))
	if err != nil {
		return mcperr.Wrap("upload_chunk", err)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := f.http.Do(req)
	if err != nil {
		return mcperr.Wrap("upload_chunk", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if e := mcperr.FromResponse(resp.StatusCode, resp.Header.Get("Retry-After"), raw); e != nil {
		return e
	}
	return nil
}

// ---- Product review screenshots -------------------------------------------

// GetIAPReviewScreenshot returns the IAP's current review screenshot id, or ""
// if none exists.
func (f *Facade) GetIAPReviewScreenshot(ctx context.Context, iapID string) (string, error) {
	resp, err := f.c.InAppPurchasesV2AppStoreReviewScreenshotGetToOneRelatedWithResponse(ctx, iapID,
		&InAppPurchasesV2AppStoreReviewScreenshotGetToOneRelatedParams{})
	if err != nil {
		return "", mcperr.Wrap("get_iap_review_screenshot", err)
	}
	return relatedID(resp.StatusCode(), resp.Body), nil
}

func (f *Facade) GetSubscriptionReviewScreenshot(ctx context.Context, subID string) (string, error) {
	resp, err := f.c.SubscriptionsAppStoreReviewScreenshotGetToOneRelatedWithResponse(ctx, subID,
		&SubscriptionsAppStoreReviewScreenshotGetToOneRelatedParams{})
	if err != nil {
		return "", mcperr.Wrap("get_subscription_review_screenshot", err)
	}
	return relatedID(resp.StatusCode(), resp.Body), nil
}

func (f *Facade) DeleteIAPReviewScreenshot(ctx context.Context, id string) error {
	resp, err := f.c.InAppPurchaseAppStoreReviewScreenshotsDeleteInstanceWithResponse(ctx, id)
	if err != nil {
		return mcperr.Wrap("delete_iap_review_screenshot", err)
	}
	return parseStatus("delete_iap_review_screenshot", resp.StatusCode(), resp.HTTPResponse.Header, resp.Body)
}

func (f *Facade) DeleteSubscriptionReviewScreenshot(ctx context.Context, id string) error {
	resp, err := f.c.SubscriptionAppStoreReviewScreenshotsDeleteInstanceWithResponse(ctx, id)
	if err != nil {
		return mcperr.Wrap("delete_subscription_review_screenshot", err)
	}
	return parseStatus("delete_subscription_review_screenshot", resp.StatusCode(), resp.HTTPResponse.Header, resp.Body)
}

// relatedID extracts a to-one related resource id from a response body, or ""
// when the relationship is empty / not found.
func relatedID(status int, raw []byte) string {
	if status == http.StatusNotFound || status < 200 || status >= 300 {
		return ""
	}
	var doc singleDoc
	if json.Unmarshal(raw, &doc) != nil {
		return ""
	}
	return doc.Data.ID
}


func (f *Facade) ReserveIAPReviewScreenshot(ctx context.Context, payload any) (Resource, error) {
	rdr, err := body(payload)
	if err != nil {
		return Resource{}, mcperr.Wrap("reserve_iap_review_screenshot", err)
	}
	resp, err := f.c.InAppPurchaseAppStoreReviewScreenshotsCreateInstanceWithBodyWithResponse(ctx, contentTypeJSON, rdr)
	if err != nil {
		return Resource{}, mcperr.Wrap("reserve_iap_review_screenshot", err)
	}
	return parseOne("reserve_iap_review_screenshot", resp.StatusCode(), resp.HTTPResponse.Header, resp.Body)
}

func (f *Facade) CommitIAPReviewScreenshot(ctx context.Context, id string, payload any) (Resource, error) {
	rdr, err := body(payload)
	if err != nil {
		return Resource{}, mcperr.Wrap("commit_iap_review_screenshot", err)
	}
	resp, err := f.c.InAppPurchaseAppStoreReviewScreenshotsUpdateInstanceWithBodyWithResponse(ctx, id, contentTypeJSON, rdr)
	if err != nil {
		return Resource{}, mcperr.Wrap("commit_iap_review_screenshot", err)
	}
	return parseOne("commit_iap_review_screenshot", resp.StatusCode(), resp.HTTPResponse.Header, resp.Body)
}

func (f *Facade) ReserveSubscriptionReviewScreenshot(ctx context.Context, payload any) (Resource, error) {
	rdr, err := body(payload)
	if err != nil {
		return Resource{}, mcperr.Wrap("reserve_subscription_review_screenshot", err)
	}
	resp, err := f.c.SubscriptionAppStoreReviewScreenshotsCreateInstanceWithBodyWithResponse(ctx, contentTypeJSON, rdr)
	if err != nil {
		return Resource{}, mcperr.Wrap("reserve_subscription_review_screenshot", err)
	}
	return parseOne("reserve_subscription_review_screenshot", resp.StatusCode(), resp.HTTPResponse.Header, resp.Body)
}

func (f *Facade) CommitSubscriptionReviewScreenshot(ctx context.Context, id string, payload any) (Resource, error) {
	rdr, err := body(payload)
	if err != nil {
		return Resource{}, mcperr.Wrap("commit_subscription_review_screenshot", err)
	}
	resp, err := f.c.SubscriptionAppStoreReviewScreenshotsUpdateInstanceWithBodyWithResponse(ctx, id, contentTypeJSON, rdr)
	if err != nil {
		return Resource{}, mcperr.Wrap("commit_subscription_review_screenshot", err)
	}
	return parseOne("commit_subscription_review_screenshot", resp.StatusCode(), resp.HTTPResponse.Header, resp.Body)
}

// ---- Submission -----------------------------------------------------------

func (f *Facade) CreateReviewSubmission(ctx context.Context, payload any) (Resource, error) {
	rdr, err := body(payload)
	if err != nil {
		return Resource{}, mcperr.Wrap("create_review_submission", err)
	}
	resp, err := f.c.ReviewSubmissionsCreateInstanceWithBodyWithResponse(ctx, contentTypeJSON, rdr)
	if err != nil {
		return Resource{}, mcperr.Wrap("create_review_submission", err)
	}
	return parseOne("create_review_submission", resp.StatusCode(), resp.HTTPResponse.Header, resp.Body)
}

func (f *Facade) GetReviewSubmission(ctx context.Context, id string) (Resource, error) {
	resp, err := f.c.ReviewSubmissionsGetInstanceWithResponse(ctx, id, &ReviewSubmissionsGetInstanceParams{})
	if err != nil {
		return Resource{}, mcperr.Wrap("get_submission_status", err)
	}
	return parseOne("get_submission_status", resp.StatusCode(), resp.HTTPResponse.Header, resp.Body)
}

func (f *Facade) UpdateReviewSubmission(ctx context.Context, id string, payload any) (Resource, error) {
	rdr, err := body(payload)
	if err != nil {
		return Resource{}, mcperr.Wrap("submit_for_review", err)
	}
	resp, err := f.c.ReviewSubmissionsUpdateInstanceWithBodyWithResponse(ctx, id, contentTypeJSON, rdr)
	if err != nil {
		return Resource{}, mcperr.Wrap("submit_for_review", err)
	}
	return parseOne("submit_for_review", resp.StatusCode(), resp.HTTPResponse.Header, resp.Body)
}

func (f *Facade) GetAppReviewSubmissions(ctx context.Context, appID string) ([]Resource, error) {
	resp, err := f.c.AppsReviewSubmissionsGetToManyRelatedWithResponse(ctx, appID, &AppsReviewSubmissionsGetToManyRelatedParams{})
	if err != nil {
		return nil, mcperr.Wrap("list_review_submissions", err)
	}
	return parseMany("list_review_submissions", resp.StatusCode(), resp.HTTPResponse.Header, resp.Body)
}

func (f *Facade) CreateReviewSubmissionItem(ctx context.Context, payload any) (Resource, error) {
	rdr, err := body(payload)
	if err != nil {
		return Resource{}, mcperr.Wrap("add_version_to_submission", err)
	}
	resp, err := f.c.ReviewSubmissionItemsCreateInstanceWithBodyWithResponse(ctx, contentTypeJSON, rdr)
	if err != nil {
		return Resource{}, mcperr.Wrap("add_version_to_submission", err)
	}
	return parseOne("add_version_to_submission", resp.StatusCode(), resp.HTTPResponse.Header, resp.Body)
}
