package client

import (
	"context"
	"net/http"
	"net/url"
	"strings"
)

// Solutions are the Tael Managed rows — a database, monitoring, object
// storage, backups, the security baseline, secrets — installed on the
// workspace's runtime and connected to apps. The API's views are explicit
// and carry nothing of the platform underneath; neither does this file.

type SolutionApp struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type SolutionBinding struct {
	AppID     string   `json:"app_id"`
	AppName   string   `json:"app_name"`
	EnvPrefix string   `json:"env_prefix"`
	Status    string   `json:"status"`
	Variables []string `json:"variables"`
	Reason    string   `json:"reason"`
	CreatedAt string   `json:"created_at"`
}

type ConnectionVariable struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Secret bool   `json:"secret"`
}

type Solution struct {
	ID              string               `json:"id"`
	SolutionKey     string               `json:"solution_key"`
	Name            string               `json:"name"`
	Category        string               `json:"category"`
	Description     string               `json:"description"`
	Includes        []string             `json:"includes"`
	Instance        string               `json:"instance"`
	Preset          string               `json:"preset"`
	PresetLabel     string               `json:"preset_label"`
	Status          string               `json:"status"`
	Stage           string               `json:"stage"`
	Included        bool                 `json:"included"`
	Badge           string               `json:"badge"`
	App             *SolutionApp         `json:"app"`
	Connection      []ConnectionVariable `json:"connection"`
	Bindings        []SolutionBinding    `json:"bindings"`
	Checks          []HealthCheck        `json:"checks"`
	Reason          string               `json:"reason"`
	UpdateAvailable bool                 `json:"update_available"`
	InstalledAt     string               `json:"installed_at"`
	ReadyAt         string               `json:"ready_at"`
	CreatedAt       string               `json:"created_at"`
}

type ListSolutionsResponse struct {
	Solutions []Solution `json:"solutions"`
}

type SolutionPreset struct {
	Key         string `json:"key"`
	Label       string `json:"label"`
	Summary     string `json:"summary"`
	StorageCost string `json:"storage_cost"`
	Available   bool   `json:"available"`
	Reason      string `json:"reason"`
}

type SolutionAvailability struct {
	State  string `json:"state"`
	Label  string `json:"label"`
	Reason string `json:"reason"`
}

type CatalogEntry struct {
	Key           string               `json:"key"`
	Name          string               `json:"name"`
	Category      string               `json:"category"`
	Description   string               `json:"description"`
	Cardinality   string               `json:"cardinality"`
	Requires      []string             `json:"requires"`
	Included      bool                 `json:"included"`
	Availability  SolutionAvailability `json:"availability"`
	DefaultPreset string               `json:"default_preset"`
	Presets       []SolutionPreset     `json:"presets"`
}

type SolutionCatalogResponse struct {
	Promise   string         `json:"promise"`
	Plan      string         `json:"plan"`
	Solutions []CatalogEntry `json:"solutions"`
}

type InstallSolutionRequest struct {
	SolutionKey string `json:"solution_key"`
	Preset      string `json:"preset,omitempty"`
	AppID       string `json:"app_id,omitempty"`
	Name        string `json:"name,omitempty"`
}

type InstallSolutionResponse struct {
	ID          string   `json:"id"`
	OperationID string   `json:"operation_id"`
	Required    []string `json:"required"`
}

type PodCheck struct {
	Name     string `json:"name"`
	Phase    string `json:"phase"`
	Ready    string `json:"ready"`
	Restarts int    `json:"restarts"`
}

type SolutionStatusResponse struct {
	Status  string        `json:"status"`
	Stage   string        `json:"stage"`
	Healthy bool          `json:"healthy"`
	Checks  []HealthCheck `json:"checks"`
	Pods    []PodCheck    `json:"pods"`
}

type SolutionOperationResponse struct {
	ID          string `json:"id"`
	OperationID string `json:"operation_id"`
	Status      string `json:"status"`
}

const solutionsPath = "/api/v1/solutions"

func solutionPath(solutionID string, suffix string) string {
	return solutionsPath + "/" + url.PathEscape(solutionID) + suffix
}

// GetSolutionCatalog returns what the workspace can add, gated by its plan.
func (client *Client) GetSolutionCatalog(requestContext context.Context) (*SolutionCatalogResponse, error) {
	var catalog SolutionCatalogResponse
	if requestError := client.doJSON(requestContext, http.MethodGet, solutionsPath+"/catalog", nil, &catalog); requestError != nil {
		return nil, requestError
	}
	return &catalog, nil
}

// ListSolutions returns every installed solution in the workspace.
func (client *Client) ListSolutions(requestContext context.Context) (*ListSolutionsResponse, error) {
	var listResponse ListSolutionsResponse
	if requestError := client.doJSON(requestContext, http.MethodGet, solutionsPath, nil, &listResponse); requestError != nil {
		return nil, requestError
	}
	return &listResponse, nil
}

// GetSolution returns one installed solution by id.
func (client *Client) GetSolution(requestContext context.Context, solutionID string) (*Solution, error) {
	var solution Solution
	if requestError := client.doJSON(requestContext, http.MethodGet, solutionPath(solutionID, ""), nil, &solution); requestError != nil {
		return nil, requestError
	}
	return &solution, nil
}

// InstallSolution starts an install. The API refuses with 402 when the
// plan does not include it, 409 when one is already installed, and 422
// when there is no room; each carries a sentence the CLI prints as is.
func (client *Client) InstallSolution(requestContext context.Context, request InstallSolutionRequest) (*InstallSolutionResponse, error) {
	var installResponse InstallSolutionResponse
	if requestError := client.doJSON(requestContext, http.MethodPost, solutionsPath, request, &installResponse); requestError != nil {
		return nil, requestError
	}
	return &installResponse, nil
}

// GetSolutionStatus reads the solution's live status from the runtime.
func (client *Client) GetSolutionStatus(requestContext context.Context, solutionID string) (*SolutionStatusResponse, error) {
	var statusResponse SolutionStatusResponse
	if requestError := client.doJSON(requestContext, http.MethodGet, solutionPath(solutionID, "/status"), nil, &statusResponse); requestError != nil {
		return nil, requestError
	}
	return &statusResponse, nil
}

// BindSolution connects the solution to an app: the app reads the
// connection as environment variables on its next deploy.
func (client *Client) BindSolution(requestContext context.Context, solutionID string, appID string) (*SolutionBinding, error) {
	var binding SolutionBinding
	body := map[string]string{"app_id": appID}
	if requestError := client.doJSON(requestContext, http.MethodPost, solutionPath(solutionID, "/bindings"), body, &binding); requestError != nil {
		return nil, requestError
	}
	return &binding, nil
}

// RemoveSolution removes the solution; with force, connected apps are
// disconnected first instead of the API refusing.
func (client *Client) RemoveSolution(requestContext context.Context, solutionID string, force bool) (*SolutionOperationResponse, error) {
	path := solutionPath(solutionID, "")
	if force {
		path += "?force=true"
	}
	var operation SolutionOperationResponse
	if requestError := client.doJSON(requestContext, http.MethodDelete, path, nil, &operation); requestError != nil {
		return nil, requestError
	}
	return &operation, nil
}

// UpgradeSolution applies the newer version Tael publishes. 409 when
// there is none, the solution is included with the runtime, or it is busy.
func (client *Client) UpgradeSolution(requestContext context.Context, solutionID string) (*SolutionOperationResponse, error) {
	var operation SolutionOperationResponse
	if requestError := client.doJSON(requestContext, http.MethodPost, solutionPath(solutionID, "/upgrade"), struct{}{}, &operation); requestError != nil {
		return nil, requestError
	}
	return &operation, nil
}

// RetrySolution runs a failed install again. 409 when it did not fail.
func (client *Client) RetrySolution(requestContext context.Context, solutionID string) (*SolutionOperationResponse, error) {
	var operation SolutionOperationResponse
	if requestError := client.doJSON(requestContext, http.MethodPost, solutionPath(solutionID, "/retry"), struct{}{}, &operation); requestError != nil {
		return nil, requestError
	}
	return &operation, nil
}

// SolutionConnectionResponse is a solution's connection as a token may
// see it: names, and values with the secrets masked. Revealing values is
// a browser action, so Revealed is always false here.
type SolutionConnectionResponse struct {
	Variables []ConnectionVariable `json:"variables"`
	Revealed  bool                 `json:"revealed"`
}

// GetSolutionConnection reads the masked connection summary.
func (client *Client) GetSolutionConnection(requestContext context.Context, solutionID string) (*SolutionConnectionResponse, error) {
	var connection SolutionConnectionResponse
	if requestError := client.doJSON(requestContext, http.MethodGet, solutionPath(solutionID, "/connection"), nil, &connection); requestError != nil {
		return nil, requestError
	}
	return &connection, nil
}

// MatchesSolution says whether a person's word for a solution — its id,
// its instance name, or its display name — names this one. Display names
// match case-insensitively, so "tael managed postgres for web" works.
func MatchesSolution(solution Solution, word string) bool {
	trimmed := strings.TrimSpace(word)
	if trimmed == "" {
		return false
	}
	return solution.ID == trimmed || solution.Instance == trimmed || strings.EqualFold(solution.Name, trimmed)
}
