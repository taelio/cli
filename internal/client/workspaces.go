package client

import (
	"context"
	"net/http"
)

// The workspaces a person belongs to. A token is made inside one
// workspace and acts there; the list says which, and what else there is.

type Membership struct {
	ID   string `json:"id"`
	Slug string `json:"slug"`
	Name string `json:"name"`
	Plan string `json:"plan"`
	Role string `json:"role"`
}

type ListWorkspacesResponse struct {
	Workspaces []Membership `json:"workspaces"`
	CurrentID  string       `json:"current_id"`
}

// ListWorkspaces returns every workspace the person is in, and which one
// this token acts in.
func (client *Client) ListWorkspaces(requestContext context.Context) (*ListWorkspacesResponse, error) {
	var listResponse ListWorkspacesResponse
	if requestError := client.doJSON(requestContext, http.MethodGet, "/api/v1/workspaces", nil, &listResponse); requestError != nil {
		return nil, requestError
	}
	return &listResponse, nil
}
