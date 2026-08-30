package client

import (
	"context"
	"net/http"
)

// What the workspace has used this period, against what its plan or
// coupon includes. Every part is optional on the wire: an older API, or
// a meter Tael does not keep yet, leaves it nil and the CLI says less.

// UsageMeter is one counted thing against its allowance. Included 0 means
// no figure was given (no limit, or none known).
type UsageMeter struct {
	Used     int64 `json:"used"`
	Included int64 `json:"included"`
}

// UsageAITokens is the AI meter: the tokens used, the allowance, where the
// allowance comes from (plan or coupon), and how many of the used tokens
// were estimated rather than reported.
type UsageAITokens struct {
	Used      int64  `json:"used"`
	Included  int64  `json:"included"`
	Source    string `json:"source"`
	Estimated int64  `json:"estimated"`
}

// UsageCustomDomains says whether the plan allows custom domains and how
// many apps have one.
type UsageCustomDomains struct {
	Allowed bool  `json:"allowed"`
	Used    int64 `json:"used"`
}

// UsageSummary is GET /api/v1/usage: the period (the first day of the
// month) and one meter per counted thing.
type UsageSummary struct {
	Period        string              `json:"period"`
	AITokens      *UsageAITokens      `json:"ai_tokens"`
	Apps          *UsageMeter         `json:"apps"`
	Seats         *UsageMeter         `json:"seats"`
	CustomDomains *UsageCustomDomains `json:"custom_domains"`
	Deploys       *int64              `json:"deploys"`
	Builds        *int64              `json:"builds"`
	Machines      *int64              `json:"machines"`
}

// GetUsage reads the workspace's usage for the current period.
func (client *Client) GetUsage(requestContext context.Context) (*UsageSummary, error) {
	var summary UsageSummary
	if requestError := client.doJSON(requestContext, http.MethodGet, "/api/v1/usage", nil, &summary); requestError != nil {
		return nil, requestError
	}
	return &summary, nil
}
