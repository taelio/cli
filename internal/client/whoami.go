package client

import (
	"context"
	"net/http"
)

type WhoamiUser struct {
	ID          string `json:"id"`
	GithubLogin string `json:"github_login"`
	Name        string `json:"name"`
}

type WhoamiWorkspace struct {
	ID   string `json:"id"`
	Slug string `json:"slug"`
	Name string `json:"name"`
	Plan string `json:"plan"`
}

type WhoamiResponse struct {
	User      WhoamiUser      `json:"user"`
	Workspace WhoamiWorkspace `json:"workspace"`
}

// Whoami returns the authenticated user and their workspace.
func (client *Client) Whoami(requestContext context.Context) (*WhoamiResponse, error) {
	var whoamiResponse WhoamiResponse
	if requestError := client.doJSON(requestContext, http.MethodGet, "/api/v1/whoami", nil, &whoamiResponse); requestError != nil {
		return nil, requestError
	}
	return &whoamiResponse, nil
}
