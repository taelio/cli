package client

import (
	"context"
	"net/http"
	"net/url"
)

// Personal access tokens: what `tael login` makes, and what scripts use.
// A token's secret is returned once, at creation, and never again.

type Token struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	Scopes     []string `json:"scopes"`
	CreatedAt  string   `json:"created_at"`
	ExpiresAt  *string  `json:"expires_at"`
	RevokedAt  *string  `json:"revoked_at"`
	LastUsedAt *string  `json:"last_used_at"`
}

type ListTokensResponse struct {
	Tokens []Token `json:"tokens"`
}

type CreateTokenRequest struct {
	Name      string  `json:"name"`
	ExpiresAt *string `json:"expires_at,omitempty"`
}

// CreatedToken carries the secret, the only time it exists in readable
// form.
type CreatedToken struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Token     string `json:"token"`
	ExpiresAt string `json:"expires_at"`
}

// ListTokens returns the caller's tokens in this workspace, without
// their secrets.
func (client *Client) ListTokens(requestContext context.Context) (*ListTokensResponse, error) {
	var listResponse ListTokensResponse
	if requestError := client.doJSON(requestContext, http.MethodGet, "/api/v1/account/tokens", nil, &listResponse); requestError != nil {
		return nil, requestError
	}
	return &listResponse, nil
}

// CreateToken makes a token for the caller in this workspace; it expires
// after 90 days unless told otherwise.
func (client *Client) CreateToken(requestContext context.Context, request CreateTokenRequest) (*CreatedToken, error) {
	var created CreatedToken
	if requestError := client.doJSON(requestContext, http.MethodPost, "/api/v1/account/tokens", request, &created); requestError != nil {
		return nil, requestError
	}
	return &created, nil
}

// RevokeToken makes a token stop working. 404 when it is unknown or
// already revoked.
func (client *Client) RevokeToken(requestContext context.Context, tokenID string) error {
	return client.doJSON(requestContext, http.MethodPost, "/api/v1/account/tokens/"+url.PathEscape(tokenID)+"/revoke", struct{}{}, nil)
}
