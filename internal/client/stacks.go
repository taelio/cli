package client

import (
	"context"
	"net/http"
	"net/url"
)

// Stacks: named groups of apps that ship together. An app belongs to at
// most one stack; a workspace without stacks looks exactly as it always
// did. Removing a stack leaves its apps in place, ungrouped.

// StackApp is one app inside a stack, as the stacks list carries it.
type StackApp struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Status string `json:"status,omitempty"`
	Tone   string `json:"tone,omitempty"`
}

// Stack is one named group of apps.
type Stack struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Description string     `json:"description,omitempty"`
	Apps        []StackApp `json:"apps"`
	CreatedAt   string     `json:"created_at,omitempty"`
	UpdatedAt   string     `json:"updated_at,omitempty"`
}

// ListStacksResponse is every stack in the workspace, apps included.
type ListStacksResponse struct {
	Stacks []Stack `json:"stacks"`
}

// CreateStackRequest names a new stack, with apps in it from the start when
// AppIDs is set.
type CreateStackRequest struct {
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	AppIDs      []string `json:"app_ids,omitempty"`
}

// PatchStackRequest changes what is set and leaves the rest alone.
type PatchStackRequest struct {
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
}

const stacksPath = "/api/v1/stacks"

func stackPath(stackID string) string {
	return stacksPath + "/" + url.PathEscape(stackID)
}

// ListStacks returns every stack in the workspace with its apps.
func (client *Client) ListStacks(requestContext context.Context) (*ListStacksResponse, error) {
	var listResponse ListStacksResponse
	if requestError := client.doJSON(requestContext, http.MethodGet, stacksPath, nil, &listResponse); requestError != nil {
		return nil, requestError
	}
	return &listResponse, nil
}

// CreateStack makes a stack. The API refuses a name the workspace already
// uses.
func (client *Client) CreateStack(requestContext context.Context, request CreateStackRequest) (*Stack, error) {
	var stack Stack
	if requestError := client.doJSON(requestContext, http.MethodPost, stacksPath, request, &stack); requestError != nil {
		return nil, requestError
	}
	return &stack, nil
}

// PatchStack changes a stack's name or description.
func (client *Client) PatchStack(requestContext context.Context, stackID string, request PatchStackRequest) (*Stack, error) {
	var stack Stack
	if requestError := client.doJSON(requestContext, http.MethodPatch, stackPath(stackID), request, &stack); requestError != nil {
		return nil, requestError
	}
	return &stack, nil
}

// DeleteStack removes a stack; its apps stay, ungrouped.
func (client *Client) DeleteStack(requestContext context.Context, stackID string) error {
	return client.doJSON(requestContext, http.MethodDelete, stackPath(stackID), nil, nil)
}

// MoveAppToStack puts an app into a stack, or with a nil stackID takes it
// out of whichever one holds it.
func (client *Client) MoveAppToStack(requestContext context.Context, appID string, stackID *string) error {
	body := map[string]*string{"stack_id": stackID}
	return client.doJSON(requestContext, http.MethodPut, appPath(appID, "/stack"), body, nil)
}
