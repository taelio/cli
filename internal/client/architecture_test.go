package client

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestArchitectureEndpoints(t *testing.T) {
	var graphScope string
	var planBody map[string]string
	var applyBody struct {
		Changes []ArchitectureChange `json:"changes"`
		Scope   string               `json:"scope"`
	}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		switch {
		case request.Method == http.MethodGet && request.URL.Path == "/api/v1/architecture":
			graphScope = request.URL.Query().Get("scope")
			_, _ = writer.Write([]byte(`{"nodes":[{"id":"runtime","kind":"runtime","title":"Shared runtime","subtitle":"Shared · ready","status":"ready","tone":"live","group":null,"facts":[{"label":"Plan","value":"Free"}],"actions":[]},{"id":"service:networking","kind":"service","title":"Networking","subtitle":"Ingress, certificates, DNS","status":"up","tone":"live","group":"runtime","facts":[],"actions":[]}],"edges":[{"from":"app:app_1","to":"runtime","kind":"runs_on"}],"suggestions":[{"prompt":"Connect a repo","reason":"Nothing is running yet."}],"scope":{"kind":"workspace"},"stacks":[{"id":"st_1","name":"checkout","app_count":3}],"runtime_services_unavailable":true,"generated_at":"2026-08-29T10:00:00Z"}`))
		case request.Method == http.MethodPost && request.URL.Path == "/api/v1/architecture/plan":
			_ = json.NewDecoder(request.Body).Decode(&planBody)
			_, _ = writer.Write([]byte(`{"summary":"Tael would add a database.","changes":[{"kind":"add_solution","solution_key":"postgres","preset":"small","app_id":"app_1","title":"Add Tael Managed Postgres for web","detail":"A database.","blocked":"Available on Launch"}],"questions":[],"preview":{"nodes":[],"edges":[]},"model":"planner"}`))
		case request.Method == http.MethodPost && request.URL.Path == "/api/v1/architecture/apply":
			_ = json.NewDecoder(request.Body).Decode(&applyBody)
			_, _ = writer.Write([]byte(`{"applied":[{"change":{"kind":"add_solution","solution_key":"postgres","title":"Add Tael Managed Postgres for web","detail":"A database."},"id":"sol_1","operation_id":"op_1","task_id":"task_1"}],"refused":[{"change":{"kind":"new_app","repo":"taelio/api","title":"Connect taelio/api","detail":"Tael reads it."},"reason":"Connect the repo from New app; Tael reads it first."}]}`))
		default:
			writer.WriteHeader(http.StatusNotFound)
			_, _ = writer.Write([]byte(`{"detail":"Not found"}`))
		}
	}))
	defer server.Close()

	apiClient := New("token", server.URL, "test")
	requestContext := context.Background()

	graph, graphError := apiClient.GetArchitecture(requestContext, "")
	if graphError != nil || len(graph.Nodes) != 2 || !graph.RuntimeServicesUnavailable || graph.Suggestions[0].Prompt != "Connect a repo" {
		t.Fatalf("GetArchitecture = %+v, %v", graph, graphError)
	}
	if graphScope != "" {
		t.Fatalf("GetArchitecture with no scope sent ?scope=%q", graphScope)
	}
	if service := graph.Nodes[1]; service.Group == nil || *service.Group != "runtime" || graph.Nodes[0].Group != nil {
		t.Fatalf("groups decoded = %+v", graph.Nodes)
	}
	if _, present := graph.NodeByID("runtime"); !present {
		t.Fatalf("NodeByID(runtime) not found")
	}
	if graph.Scope == nil || graph.Scope.Kind != "workspace" || len(graph.Stacks) != 1 || graph.Stacks[0].Name != "checkout" || graph.Stacks[0].AppCount != 3 {
		t.Fatalf("scope and stacks decoded = %+v, %+v", graph.Scope, graph.Stacks)
	}

	if _, scopedError := apiClient.GetArchitecture(requestContext, StackScope("st_1")); scopedError != nil || graphScope != "stack:st_1" {
		t.Fatalf("GetArchitecture(stack scope) sent ?scope=%q, %v", graphScope, scopedError)
	}
	if _, scopedError := apiClient.GetArchitecture(requestContext, AppScope("app_1")); scopedError != nil || graphScope != "app:app_1" {
		t.Fatalf("GetArchitecture(app scope) sent ?scope=%q, %v", graphScope, scopedError)
	}

	plan, planError := apiClient.PlanArchitecture(requestContext, "Add a database", "")
	if planError != nil || plan.Summary != "Tael would add a database." || len(plan.Changes) != 1 || plan.Changes[0].Blocked != "Available on Launch" || plan.Changes[0].AppID != "app_1" {
		t.Fatalf("PlanArchitecture = %+v, %v", plan, planError)
	}
	if planBody["prompt"] != "Add a database" {
		t.Fatalf("plan request body = %v", planBody)
	}
	if _, present := planBody["scope"]; present {
		t.Fatalf("plan with no scope still sent one: %v", planBody)
	}
	if _, scopedError := apiClient.PlanArchitecture(requestContext, "Add a database", AppScope("app_1")); scopedError != nil || planBody["scope"] != "app:app_1" {
		t.Fatalf("plan scope sent = %v, %v", planBody, scopedError)
	}

	outcome, applyError := apiClient.ApplyArchitecture(requestContext, plan.Changes, StackScope("st_1"))
	if applyError != nil || len(outcome.Applied) != 1 || outcome.Applied[0].TaskID != "task_1" || len(outcome.Refused) != 1 || outcome.Refused[0].Reason == "" {
		t.Fatalf("ApplyArchitecture = %+v, %v", outcome, applyError)
	}
	if len(applyBody.Changes) != 1 || applyBody.Changes[0].SolutionKey != "postgres" || applyBody.Changes[0].Blocked != "Available on Launch" || applyBody.Scope != "stack:st_1" {
		t.Fatalf("apply request body = %+v", applyBody)
	}
}

// 501 is the one answer with a fixed meaning: no model on this deployment.
// It becomes ErrPlanningUnavailable so the CLI prints the product's sentence
// and nothing else; every other refusal keeps the API's sentence.
func TestPlanArchitectureRefusals(t *testing.T) {
	status := http.StatusNotImplemented
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(status)
		_, _ = writer.Write([]byte(`{"detail":"Tael could not write a plan just now; try again in a minute."}`))
	}))
	defer server.Close()
	apiClient := New("token", server.URL, "test")

	_, unavailable := apiClient.PlanArchitecture(context.Background(), "Add a database", "")
	if !errors.Is(unavailable, ErrPlanningUnavailable) {
		t.Fatalf("PlanArchitecture on 501 = %v, want ErrPlanningUnavailable", unavailable)
	}

	status = http.StatusBadGateway
	_, down := apiClient.PlanArchitecture(context.Background(), "Add a database", "")
	var apiError *APIError
	if !errors.As(down, &apiError) || apiError.StatusCode != http.StatusBadGateway || apiError.Detail != "Tael could not write a plan just now; try again in a minute." {
		t.Fatalf("PlanArchitecture on 502 = %v, want the API's sentence", down)
	}
}

// The links endpoints declare and take back that one app calls another.
func TestArchitectureLinks(t *testing.T) {
	var createBody, deleteBody map[string]string
	emptyCreateAnswer := false
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		switch {
		case request.Method == http.MethodPost && request.URL.Path == "/api/v1/architecture/links":
			createBody = nil
			_ = json.NewDecoder(request.Body).Decode(&createBody)
			writer.WriteHeader(http.StatusCreated)
			if !emptyCreateAnswer {
				_, _ = writer.Write([]byte(`{"id":"lnk_1","from_app_id":"app_1","to_app_id":"app_2","label":"REST","created_at":"2026-08-30T10:00:00Z"}`))
			}
		case request.Method == http.MethodDelete && request.URL.Path == "/api/v1/architecture/links":
			_ = json.NewDecoder(request.Body).Decode(&deleteBody)
			writer.WriteHeader(http.StatusNoContent)
		default:
			writer.WriteHeader(http.StatusNotFound)
			_, _ = writer.Write([]byte(`{"detail":"Not found"}`))
		}
	}))
	defer server.Close()
	apiClient := New("token", server.URL, "test")
	requestContext := context.Background()

	link, createError := apiClient.CreateArchitectureLink(requestContext, "app_1", "app_2", "REST")
	if createError != nil || link.ID != "lnk_1" || link.FromAppID != "app_1" || link.Label != "REST" {
		t.Fatalf("CreateArchitectureLink = %+v, %v", link, createError)
	}
	if createBody["from_app_id"] != "app_1" || createBody["to_app_id"] != "app_2" || createBody["label"] != "REST" {
		t.Fatalf("create link body = %v", createBody)
	}

	// No label: the field stays out of the request. A 201 with no body is
	// still a made link.
	emptyCreateAnswer = true
	bare, bareError := apiClient.CreateArchitectureLink(requestContext, "app_1", "app_2", "")
	if bareError != nil || bare.ID != "" {
		t.Fatalf("CreateArchitectureLink with an empty answer = %+v, %v", bare, bareError)
	}
	if _, present := createBody["label"]; present {
		t.Fatalf("create link with no label still sent one: %v", createBody)
	}

	if deleteError := apiClient.DeleteArchitectureLink(requestContext, "app_1", "app_2"); deleteError != nil {
		t.Fatalf("DeleteArchitectureLink: %v", deleteError)
	}
	if deleteBody["from_app_id"] != "app_1" || deleteBody["to_app_id"] != "app_2" {
		t.Fatalf("delete link body = %v", deleteBody)
	}
}
