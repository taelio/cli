package client

import (
	"context"
	"net/http"
	"net/url"
)

type App struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	RepoFullName string `json:"repo_full_name"`
	Status       string `json:"status"`
	LiveURL      string `json:"live_url"`
	UpdatedAt    string `json:"updated_at"`
}

type ListAppsResponse struct {
	Apps []App `json:"apps"`
}

type AppDetail struct {
	ID            string      `json:"id"`
	Name          string      `json:"name"`
	RepoFullName  string      `json:"repo_full_name"`
	Status        string      `json:"status"`
	LiveURL       string      `json:"live_url"`
	PipelineStage string      `json:"pipeline_stage"`
	LastDeploy    *LastDeploy `json:"last_deploy"`
}

type LastDeploy struct {
	ID            string `json:"id"`
	Status        string `json:"status"`
	CommitSHA     string `json:"commit_sha"`
	CommitMessage string `json:"commit_message"`
	CreatedAt     string `json:"created_at"`
}

type HealthCheck struct {
	Name    string `json:"name"`
	Status  string `json:"status"`
	Message string `json:"message"`
}

type AppStatusResponse struct {
	Status  string        `json:"status"`
	LiveURL string        `json:"live_url"`
	Healthy bool          `json:"healthy"`
	Checks  []HealthCheck `json:"checks"`
}

// appPath joins an app-scoped endpoint path, escaping the user-supplied app
// ID or name.
func appPath(appIDOrName string, suffix string) string {
	return "/api/v1/apps/" + url.PathEscape(appIDOrName) + suffix
}

// ListApps returns every app in the workspace.
func (client *Client) ListApps(requestContext context.Context) (*ListAppsResponse, error) {
	var listResponse ListAppsResponse
	if requestError := client.doJSON(requestContext, http.MethodGet, "/api/v1/apps", nil, &listResponse); requestError != nil {
		return nil, requestError
	}
	return &listResponse, nil
}

// GetApp returns one app by ID or name.
func (client *Client) GetApp(requestContext context.Context, appIDOrName string) (*AppDetail, error) {
	var detailResponse AppDetail
	if requestError := client.doJSON(requestContext, http.MethodGet, appPath(appIDOrName, ""), nil, &detailResponse); requestError != nil {
		return nil, requestError
	}
	return &detailResponse, nil
}

// GetAppStatus returns the live status and health checks for one app.
func (client *Client) GetAppStatus(requestContext context.Context, appIDOrName string) (*AppStatusResponse, error) {
	var statusResponse AppStatusResponse
	if requestError := client.doJSON(requestContext, http.MethodGet, appPath(appIDOrName, "/status"), nil, &statusResponse); requestError != nil {
		return nil, requestError
	}
	return &statusResponse, nil
}
