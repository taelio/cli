package client

import (
	"context"
	"net/http"
)

type Deploy struct {
	ID            string `json:"id"`
	Status        string `json:"status"`
	CommitSHA     string `json:"commit_sha"`
	CommitMessage string `json:"commit_message"`
	ImageTag      string `json:"image_tag"`
	CreatedAt     string `json:"created_at"`
	FinishedAt    string `json:"finished_at"`
}

type ListDeploysResponse struct {
	Deploys []Deploy `json:"deploys"`
}

type CreateDeployResponse struct {
	DeployID string `json:"deploy_id"`
}

// CreateDeploy triggers a new deploy of the app's tracked branch.
func (client *Client) CreateDeploy(requestContext context.Context, appIDOrName string) (*CreateDeployResponse, error) {
	var createResponse CreateDeployResponse
	if requestError := client.doJSON(requestContext, http.MethodPost, appPath(appIDOrName, "/deploys"), struct{}{}, &createResponse); requestError != nil {
		return nil, requestError
	}
	return &createResponse, nil
}

// ListDeploys returns the deploy history for one app, newest first.
func (client *Client) ListDeploys(requestContext context.Context, appIDOrName string) (*ListDeploysResponse, error) {
	var listResponse ListDeploysResponse
	if requestError := client.doJSON(requestContext, http.MethodGet, appPath(appIDOrName, "/deploys"), nil, &listResponse); requestError != nil {
		return nil, requestError
	}
	return &listResponse, nil
}
