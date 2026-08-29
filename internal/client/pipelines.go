package client

import (
	"context"
	"net/http"
)

// A pipeline is the graph of steps that takes a push to a live app: what
// triggers it, the steps, and the order they run in. Editing it queues a
// pull request with the change; the app's repository stays the source of
// truth.

type PipelineTriggers struct {
	PushBranches []string `json:"push_branches,omitempty"`
	PullRequest  bool     `json:"pull_request,omitempty"`
}

type PipelineNode struct {
	ID   string `json:"id"`
	Kind string `json:"kind"`
	Name string `json:"name,omitempty"`
	// With carries the step's settings (a path to probe, an environment).
	With map[string]string `json:"with,omitempty"`
	Env  map[string]string `json:"env,omitempty"`
	// Managed steps are Tael's own: their settings can change, the step
	// itself cannot be removed.
	Managed        bool   `json:"managed,omitempty"`
	RawYAML        string `json:"raw_yaml,omitempty"`
	TimeoutMinutes int    `json:"timeout_minutes,omitempty"`
}

type PipelineEdge struct {
	From string `json:"from"`
	To   string `json:"to"`
}

type PipelineGraph struct {
	Schema   int              `json:"schema"`
	Triggers PipelineTriggers `json:"triggers"`
	Nodes    []PipelineNode   `json:"nodes"`
	Edges    []PipelineEdge   `json:"edges"`
}

type Pipeline struct {
	ID           string        `json:"id"`
	GraphVersion int           `json:"graph_version"`
	Graph        PipelineGraph `json:"graph"`
}

type PipelineUpdateResponse struct {
	Status       string `json:"status"`
	RevisionID   string `json:"revision_id"`
	GraphVersion int    `json:"graph_version"`
	Note         string `json:"note"`
}

// GetPipeline reads the app's pipeline, creating the default one when the
// app has none yet.
func (client *Client) GetPipeline(requestContext context.Context, appIDOrName string) (*Pipeline, error) {
	var pipeline Pipeline
	if requestError := client.doJSON(requestContext, http.MethodGet, appPath(appIDOrName, "/pipeline"), nil, &pipeline); requestError != nil {
		return nil, requestError
	}
	return &pipeline, nil
}

// PutPipeline saves an edited graph. graphVersion is the version the edit
// was made against; the API refuses with 409 when it moved on.
func (client *Client) PutPipeline(requestContext context.Context, appIDOrName string, graph PipelineGraph, graphVersion int) (*PipelineUpdateResponse, error) {
	body := struct {
		Graph        PipelineGraph `json:"graph"`
		GraphVersion int           `json:"graph_version"`
	}{Graph: graph, GraphVersion: graphVersion}
	var response PipelineUpdateResponse
	if requestError := client.doJSON(requestContext, http.MethodPut, appPath(appIDOrName, "/pipeline"), body, &response); requestError != nil {
		return nil, requestError
	}
	return &response, nil
}
