package client

import (
	"context"
	"net/http"
)

// The workspace's plan: what the status summary says, and what a coupon
// granted.

// WorkspaceEnvironment is the production runtime in the words the UI uses.
type WorkspaceEnvironment struct {
	Status         string  `json:"status"`
	Tier           *string `json:"tier"`
	Kind           *string `json:"kind"`
	BaseDomain     *string `json:"base_domain"`
	PendingRuntime *string `json:"pending_cluster"`
	UpgradeError   *string `json:"upgrade_error"`
}

type WorkspaceStatus struct {
	WorkspaceStatus string                `json:"workspace_status"`
	Plan            string                `json:"plan"`
	AppsTotal       int                   `json:"apps_total"`
	AppsLive        int                   `json:"apps_live"`
	OpenIncidents   int                   `json:"open_incidents"`
	NeedsYou        int                   `json:"needs_you"`
	RuntimeStatus   *string               `json:"runtime_status"`
	Environment     *WorkspaceEnvironment `json:"environment"`
}

// CouponGrant is what a redeemed coupon put in place, in plain terms.
type CouponGrant struct {
	Code             string `json:"code"`
	Plan             string `json:"plan"`
	Until            string `json:"until"`
	AppsIncluded     int    `json:"apps_included"`
	AITokensIncluded int64  `json:"ai_tokens_included"`
}

// AppliedCoupon is the coupon in force on the workspace, as GET
// /api/v1/workspace/coupon reports it while the grant lasts.
type AppliedCoupon struct {
	Code             string `json:"code"`
	Plan             string `json:"plan"`
	GrantedUntil     string `json:"granted_until"`
	AppsIncluded     int    `json:"apps_included"`
	AITokensIncluded int64  `json:"ai_tokens_included"`
	RedeemedAt       string `json:"redeemed_at"`
}

type CouponGrantResponse struct {
	Grant   *CouponGrant   `json:"grant"`
	Applied *AppliedCoupon `json:"applied,omitempty"`
}

// GetWorkspaceStatus reads the workspace summary: plan, apps, incidents,
// what needs a person, and the runtime.
func (client *Client) GetWorkspaceStatus(requestContext context.Context) (*WorkspaceStatus, error) {
	var status WorkspaceStatus
	if requestError := client.doJSON(requestContext, http.MethodGet, "/api/v1/status", nil, &status); requestError != nil {
		return nil, requestError
	}
	return &status, nil
}

// GetCouponGrant reads the coupon in force, if any.
func (client *Client) GetCouponGrant(requestContext context.Context) (*CouponGrantResponse, error) {
	var response CouponGrantResponse
	if requestError := client.doJSON(requestContext, http.MethodGet, "/api/v1/workspace/coupon", nil, &response); requestError != nil {
		return nil, requestError
	}
	return &response, nil
}

// RedeemCoupon applies a code. 404 for a code Tael does not know, 409
// when the workspace has used one or is already paid; each with a sentence.
func (client *Client) RedeemCoupon(requestContext context.Context, code string) (*CouponGrantResponse, error) {
	var response CouponGrantResponse
	if requestError := client.doJSON(requestContext, http.MethodPost, "/api/v1/workspace/coupon", map[string]string{"code": code}, &response); requestError != nil {
		return nil, requestError
	}
	return &response, nil
}
