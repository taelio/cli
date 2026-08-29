package client

import (
	"context"
	"net/http"
	"net/url"
)

type App struct {
	ID                string `json:"id"`
	Name              string `json:"name"`
	RepoFullName      string `json:"repo_full_name"`
	DefaultBranch     string `json:"default_branch"`
	Status            string `json:"status"`
	LiveURL           string `json:"live_url"`
	DetectedFramework string `json:"detected_framework"`
	DetectedGenerator string `json:"detected_generator"`
	CreatedAt         string `json:"created_at"`
	UpdatedAt         string `json:"updated_at"`
}

type ListAppsResponse struct {
	Apps []App `json:"apps"`
}

type AppDetail struct {
	ID                string      `json:"id"`
	Name              string      `json:"name"`
	RepoFullName      string      `json:"repo_full_name"`
	Status            string      `json:"status"`
	LiveURL           string      `json:"live_url"`
	PipelineStage     string      `json:"pipeline_stage"`
	DetectedFramework string      `json:"detected_framework"`
	DetectedGenerator string      `json:"detected_generator"`
	LastDeploy        *LastDeploy `json:"last_deploy"`
}

type LastDeploy struct {
	ID            string `json:"id"`
	Status        string `json:"status"`
	CommitSHA     string `json:"commit_sha"`
	CommitMessage string `json:"commit_message"`
	CreatedAt     string `json:"created_at"`
	FinishedAt    string `json:"finished_at"`
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

// AppActionResponse is what the app actions answer: the app and the
// status it is now in (removed, creating, going_live).
type AppActionResponse struct {
	ID             string `json:"id"`
	Status         string `json:"status"`
	PullRequestURL string `json:"pull_request_url"`
}

// SetupProgressEntry is one step of Tael reading the repository, in words.
type SetupProgressEntry struct {
	Key     string `json:"key"`
	Status  string `json:"status"`
	Message string `json:"message"`
}

type GeneratedFile struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

// AppSetup is where an app's setup stands: what Tael read, what it wrote,
// and the setup pull request once there is one.
type AppSetup struct {
	Status            string               `json:"status"`
	State             string               `json:"state"`
	AnalysisStatus    string               `json:"analysis_status"`
	ErrorMessage      string               `json:"error_message"`
	DetectedFramework string               `json:"detected_framework"`
	DetectedLanguage  string               `json:"detected_language"`
	CreationProgress  []SetupProgressEntry `json:"creation_progress"`
	GeneratedFiles    []GeneratedFile      `json:"generated_files"`
	PullRequestURL    string               `json:"pull_request_url"`
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

// RemoveApp takes the app out of Tael. The repository is untouched.
func (client *Client) RemoveApp(requestContext context.Context, appIDOrName string) (*AppActionResponse, error) {
	var response AppActionResponse
	if requestError := client.doJSON(requestContext, http.MethodDelete, appPath(appIDOrName, ""), nil, &response); requestError != nil {
		return nil, requestError
	}
	return &response, nil
}

// RetryApp runs a failed setup again from the step that failed. The API
// refuses with 409 when nothing failed.
func (client *Client) RetryApp(requestContext context.Context, appIDOrName string) (*AppActionResponse, error) {
	var response AppActionResponse
	if requestError := client.doJSON(requestContext, http.MethodPost, appPath(appIDOrName, "/retry"), struct{}{}, &response); requestError != nil {
		return nil, requestError
	}
	return &response, nil
}

// GoLive merges the setup pull request so the first deploy starts.
func (client *Client) GoLive(requestContext context.Context, appIDOrName string) (*AppActionResponse, error) {
	var response AppActionResponse
	if requestError := client.doJSON(requestContext, http.MethodPost, appPath(appIDOrName, "/go-live"), struct{}{}, &response); requestError != nil {
		return nil, requestError
	}
	return &response, nil
}

// GetAppSetup reads where the app's setup stands.
func (client *Client) GetAppSetup(requestContext context.Context, appIDOrName string) (*AppSetup, error) {
	var setup AppSetup
	if requestError := client.doJSON(requestContext, http.MethodGet, appPath(appIDOrName, "/setup"), nil, &setup); requestError != nil {
		return nil, requestError
	}
	return &setup, nil
}
