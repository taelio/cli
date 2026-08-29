package client

import (
	"context"
	"net/http"
)

// Onboarding: the repositories the Tael GitHub App can see, Tael's reading
// of one, and making an app from it.

type Repository struct {
	InstallationID    int64  `json:"installation_id"`
	FullName          string `json:"full_name"`
	DefaultBranch     string `json:"default_branch"`
	Private           bool   `json:"private"`
	Language          string `json:"language"`
	Description       string `json:"description"`
	PushedAt          string `json:"pushed_at"`
	DetectedGenerator string `json:"detected_generator"`
}

type ListRepositoriesResponse struct {
	Repos []Repository `json:"repos"`
}

// RepoAnalysis is what Tael read the repository to be: every field is a
// conclusion in the owner's terms, not what GitHub reported.
type RepoAnalysis struct {
	Framework           string   `json:"framework"`
	Language            string   `json:"language"`
	Summary             string   `json:"summary"`
	NeedsDatabase       bool     `json:"needs_database"`
	DatabaseReason      string   `json:"database_reason"`
	HasDockerfile       bool     `json:"has_dockerfile"`
	BuildStrategy       string   `json:"build_strategy"`
	ListensOnPort       int      `json:"listens_on_port"`
	RequiredEnvironment []string `json:"required_environment"`
	Concerns            []string `json:"concerns"`
	Confidence          string   `json:"confidence"`
	Model               string   `json:"model"`
	Partial             bool     `json:"partial"`
}

type AnalyseRepositoryRequest struct {
	RepoFullName   string `json:"repo_full_name"`
	DefaultBranch  string `json:"default_branch"`
	InstallationID int64  `json:"installation_id"`
}

type CreateAppRequest struct {
	Name           string `json:"name,omitempty"`
	RepoFullName   string `json:"repo_full_name"`
	DefaultBranch  string `json:"default_branch,omitempty"`
	InstallationID int64  `json:"installation_id"`
}

type CreateAppResponse struct {
	ID string `json:"id"`
}

// ListRepositories returns the repositories the workspace's GitHub
// installations can see.
func (client *Client) ListRepositories(requestContext context.Context) (*ListRepositoriesResponse, error) {
	var listResponse ListRepositoriesResponse
	if requestError := client.doJSON(requestContext, http.MethodGet, "/api/v1/github/repos", nil, &listResponse); requestError != nil {
		return nil, requestError
	}
	return &listResponse, nil
}

// AnalyseRepository asks Tael to read a repository before anything is
// made from it. The API answers 501 when the reading is not configured;
// the caller carries on without it.
func (client *Client) AnalyseRepository(requestContext context.Context, request AnalyseRepositoryRequest) (*RepoAnalysis, error) {
	var analysis RepoAnalysis
	if requestError := client.doJSON(requestContext, http.MethodPost, "/api/v1/repos/analyse", request, &analysis); requestError != nil {
		return nil, requestError
	}
	return &analysis, nil
}

// CreateApp makes an app from a repository and starts its setup. 409 when
// the repository is already an app.
func (client *Client) CreateApp(requestContext context.Context, request CreateAppRequest) (*CreateAppResponse, error) {
	var response CreateAppResponse
	if requestError := client.doJSON(requestContext, http.MethodPost, "/api/v1/apps", request, &response); requestError != nil {
		return nil, requestError
	}
	return &response, nil
}
