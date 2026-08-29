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
	var planBody map[string]string
	var applyBody map[string][]ArchitectureChange
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		switch {
		case request.Method == http.MethodGet && request.URL.Path == "/api/v1/architecture":
			_, _ = writer.Write([]byte(`{"nodes":[{"id":"runtime","kind":"runtime","title":"Shared runtime","subtitle":"Shared · ready","status":"ready","tone":"live","group":null,"facts":[{"label":"Plan","value":"Free"}],"actions":[]},{"id":"service:networking","kind":"service","title":"Networking","subtitle":"Ingress, certificates, DNS","status":"up","tone":"live","group":"runtime","facts":[],"actions":[]}],"edges":[{"from":"app:app_1","to":"runtime","kind":"runs_on"}],"suggestions":[{"prompt":"Connect a repo","reason":"Nothing is running yet."}],"runtime_services_unavailable":true,"generated_at":"2026-08-29T10:00:00Z"}`))
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

	graph, graphError := apiClient.GetArchitecture(requestContext)
	if graphError != nil || len(graph.Nodes) != 2 || !graph.RuntimeServicesUnavailable || graph.Suggestions[0].Prompt != "Connect a repo" {
		t.Fatalf("GetArchitecture = %+v, %v", graph, graphError)
	}
	if service := graph.Nodes[1]; service.Group == nil || *service.Group != "runtime" || graph.Nodes[0].Group != nil {
		t.Fatalf("groups decoded = %+v", graph.Nodes)
	}
	if _, present := graph.NodeByID("runtime"); !present {
		t.Fatalf("NodeByID(runtime) not found")
	}

	plan, planError := apiClient.PlanArchitecture(requestContext, "Add a database")
	if planError != nil || plan.Summary != "Tael would add a database." || len(plan.Changes) != 1 || plan.Changes[0].Blocked != "Available on Launch" || plan.Changes[0].AppID != "app_1" {
		t.Fatalf("PlanArchitecture = %+v, %v", plan, planError)
	}
	if planBody["prompt"] != "Add a database" {
		t.Fatalf("plan request body = %v", planBody)
	}

	outcome, applyError := apiClient.ApplyArchitecture(requestContext, plan.Changes)
	if applyError != nil || len(outcome.Applied) != 1 || outcome.Applied[0].TaskID != "task_1" || len(outcome.Refused) != 1 || outcome.Refused[0].Reason == "" {
		t.Fatalf("ApplyArchitecture = %+v, %v", outcome, applyError)
	}
	if sent := applyBody["changes"]; len(sent) != 1 || sent[0].SolutionKey != "postgres" || sent[0].Blocked != "Available on Launch" {
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

	_, unavailable := apiClient.PlanArchitecture(context.Background(), "Add a database")
	if !errors.Is(unavailable, ErrPlanningUnavailable) {
		t.Fatalf("PlanArchitecture on 501 = %v, want ErrPlanningUnavailable", unavailable)
	}

	status = http.StatusBadGateway
	_, down := apiClient.PlanArchitecture(context.Background(), "Add a database")
	var apiError *APIError
	if !errors.As(down, &apiError) || apiError.StatusCode != http.StatusBadGateway || apiError.Detail != "Tael could not write a plan just now; try again in a minute." {
		t.Fatalf("PlanArchitecture on 502 = %v, want the API's sentence", down)
	}
}
