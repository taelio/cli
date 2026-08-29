package client

import (
	"context"
	"errors"
	"net/http"
)

// The architecture studio: the workspace as one picture — the runtime, the
// apps on it, the solutions they read, the addresses that reach them — a
// change set Tael writes from a sentence, and the call that carries it out.
// The API's views are explicit and carry nothing of the platform
// underneath; neither does this file.

// Node kinds in the picture.
const (
	ArchitectureKindRuntime  = "runtime"
	ArchitectureKindApp      = "app"
	ArchitectureKindSolution = "solution"
	ArchitectureKindDomain   = "domain"
	ArchitectureKindService  = "service"
)

// Edge kinds in the picture.
const (
	ArchitectureEdgeRoutes   = "routes"
	ArchitectureEdgeReads    = "reads"
	ArchitectureEdgeRunsOn   = "runs_on"
	ArchitectureEdgeRequires = "requires"
)

// Change kinds a plan proposes.
const (
	ChangeAddSolution = "add_solution"
	ChangeConnect     = "connect"
	ChangeNewApp      = "new_app"
)

// ArchitectureRuntimeNodeID is the one runtime node every workspace has.
const ArchitectureRuntimeNodeID = "runtime"

// ArchitectureFact is one line on a node's card.
type ArchitectureFact struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

// ArchitectureAction is a button on a node: a link, or a deploy of the app.
type ArchitectureAction struct {
	Kind  string `json:"kind"`
	Label string `json:"label"`
	Href  string `json:"href,omitempty"`
	AppID string `json:"app_id,omitempty"`
}

// ArchitectureNode is one box in the picture.
type ArchitectureNode struct {
	ID       string               `json:"id"`
	Kind     string               `json:"kind"`
	Title    string               `json:"title"`
	Subtitle string               `json:"subtitle"`
	Status   string               `json:"status"`
	Tone     string               `json:"tone"`
	Group    *string              `json:"group"`
	Href     string               `json:"href,omitempty"`
	Facts    []ArchitectureFact   `json:"facts"`
	Actions  []ArchitectureAction `json:"actions"`
	// Proposed marks a node a plan would add; the graph itself never sets it.
	Proposed bool `json:"proposed,omitempty"`
}

// ArchitectureEdge is one line between two nodes.
type ArchitectureEdge struct {
	From     string `json:"from"`
	To       string `json:"to"`
	Kind     string `json:"kind"`
	Label    string `json:"label,omitempty"`
	Proposed bool   `json:"proposed,omitempty"`
}

// ArchitectureSuggestion is a change Tael would make, phrased as the prompt
// that asks for it.
type ArchitectureSuggestion struct {
	Prompt string `json:"prompt"`
	Reason string `json:"reason"`
}

// ArchitectureGraph is the workspace as served.
type ArchitectureGraph struct {
	Nodes       []ArchitectureNode       `json:"nodes"`
	Edges       []ArchitectureEdge       `json:"edges"`
	Suggestions []ArchitectureSuggestion `json:"suggestions"`
	// RuntimeServicesUnavailable says the runtime did not answer when asked
	// what it carries; the rest of the picture stands.
	RuntimeServicesUnavailable bool   `json:"runtime_services_unavailable,omitempty"`
	GeneratedAt                string `json:"generated_at"`
}

// NodeByID finds a node in the graph.
func (graph *ArchitectureGraph) NodeByID(id string) (ArchitectureNode, bool) {
	for _, node := range graph.Nodes {
		if node.ID == id {
			return node, true
		}
	}
	return ArchitectureNode{}, false
}

// ArchitectureChange is one thing a plan proposes; the same shape goes back
// to apply.
type ArchitectureChange struct {
	Kind        string `json:"kind"`
	SolutionKey string `json:"solution_key,omitempty"`
	Preset      string `json:"preset,omitempty"`
	AppID       string `json:"app_id,omitempty"`
	Repo        string `json:"repo,omitempty"`
	Branch      string `json:"branch,omitempty"`
	Title       string `json:"title"`
	Detail      string `json:"detail"`
	// Blocked says why the change cannot be applied as things stand
	// ("Available on Launch"); the change is still shown.
	Blocked string `json:"blocked,omitempty"`
}

// ArchitecturePreview is the picture with the proposed parts marked.
type ArchitecturePreview struct {
	Nodes []ArchitectureNode `json:"nodes"`
	Edges []ArchitectureEdge `json:"edges"`
}

// ArchitecturePlan is what the planner answers.
type ArchitecturePlan struct {
	Summary   string               `json:"summary"`
	Changes   []ArchitectureChange `json:"changes"`
	Questions []string             `json:"questions"`
	Preview   ArchitecturePreview  `json:"preview"`
	Model     string               `json:"model"`
}

// ArchitectureApplied is one change that went through: the row it made or
// touched and the work doing it.
type ArchitectureApplied struct {
	Change      ArchitectureChange `json:"change"`
	ID          string             `json:"id"`
	OperationID string             `json:"operation_id"`
	TaskID      string             `json:"task_id,omitempty"`
}

// ArchitectureRefused is one change that did not go through, and why, in a
// sentence.
type ArchitectureRefused struct {
	Change ArchitectureChange `json:"change"`
	Reason string             `json:"reason"`
}

// ArchitectureOutcome is what applying a plan did. The API stops at the
// first refusal; the changes after it are not attempted and not listed.
type ArchitectureOutcome struct {
	Applied []ArchitectureApplied `json:"applied"`
	Refused []ArchitectureRefused `json:"refused"`
}

// ErrPlanningUnavailable is the 501 answer: no model is set up on this
// deployment to write plans. The sentence is the product's.
var ErrPlanningUnavailable = errors.New("Tael cannot plan changes on this deployment yet.")

const architecturePath = "/api/v1/architecture"

// GetArchitecture reads the workspace as a picture.
func (client *Client) GetArchitecture(requestContext context.Context) (*ArchitectureGraph, error) {
	var graph ArchitectureGraph
	if requestError := client.doJSON(requestContext, http.MethodGet, architecturePath, nil, &graph); requestError != nil {
		return nil, requestError
	}
	return &graph, nil
}

// PlanArchitecture asks Tael for the smallest set of changes that does what
// the sentence asks. 501 becomes ErrPlanningUnavailable; 502 carries a
// sentence in Tael's words when the model did not answer.
func (client *Client) PlanArchitecture(requestContext context.Context, prompt string) (*ArchitecturePlan, error) {
	var plan ArchitecturePlan
	body := map[string]string{"prompt": prompt}
	if requestError := client.doJSON(requestContext, http.MethodPost, architecturePath+"/plan", body, &plan); requestError != nil {
		var apiError *APIError
		if errors.As(requestError, &apiError) && apiError.StatusCode == http.StatusNotImplemented {
			return nil, ErrPlanningUnavailable
		}
		return nil, requestError
	}
	return &plan, nil
}

// ApplyArchitecture carries the changes out in order. What was refused, and
// why, comes back in the outcome rather than as an error.
func (client *Client) ApplyArchitecture(requestContext context.Context, changes []ArchitectureChange) (*ArchitectureOutcome, error) {
	var outcome ArchitectureOutcome
	body := map[string][]ArchitectureChange{"changes": changes}
	if requestError := client.doJSON(requestContext, http.MethodPost, architecturePath+"/apply", body, &outcome); requestError != nil {
		return nil, requestError
	}
	return &outcome, nil
}
