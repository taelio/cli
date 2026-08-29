package client

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSolutionsEndpoints(t *testing.T) {
	var installBody InstallSolutionRequest
	var bindBody map[string]string
	var removePath string
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		switch {
		case request.Method == http.MethodGet && request.URL.Path == "/api/v1/solutions/catalog":
			_, _ = writer.Write([]byte(`{"promise":"Tael installs it.","plan":"launch","solutions":[{"key":"postgres","name":"Tael Managed Postgres","availability":{"state":"available","label":"Add"},"default_preset":"small","presets":[{"key":"small","label":"Small","available":true}]}]}`))
		case request.Method == http.MethodGet && request.URL.Path == "/api/v1/solutions":
			_, _ = writer.Write([]byte(`{"solutions":[{"id":"sol_1","solution_key":"postgres","name":"Tael Managed Postgres for web","instance":"tael-postgres-web","status":"ready","app":{"id":"app_1","name":"web"},"bindings":[{"app_id":"app_1","app_name":"web","status":"bound","variables":["DATABASE_URL"]}]}]}`))
		case request.Method == http.MethodPost && request.URL.Path == "/api/v1/solutions":
			_ = json.NewDecoder(request.Body).Decode(&installBody)
			writer.WriteHeader(http.StatusCreated)
			_, _ = writer.Write([]byte(`{"id":"sol_2","operation_id":"op_1","required":["sol_3"]}`))
		case request.Method == http.MethodGet && request.URL.Path == "/api/v1/solutions/sol_1/status":
			_, _ = writer.Write([]byte(`{"status":"ready","stage":"ready","healthy":true,"checks":[{"name":"solution","status":"ok","message":"Running."}],"pods":[]}`))
		case request.Method == http.MethodPost && request.URL.Path == "/api/v1/solutions/sol_1/bindings":
			_ = json.NewDecoder(request.Body).Decode(&bindBody)
			writer.WriteHeader(http.StatusCreated)
			_, _ = writer.Write([]byte(`{"app_id":"app_2","app_name":"api","status":"binding","variables":[]}`))
		case request.Method == http.MethodDelete && request.URL.Path == "/api/v1/solutions/sol_1":
			removePath = request.URL.RequestURI()
			writer.WriteHeader(http.StatusAccepted)
			_, _ = writer.Write([]byte(`{"id":"sol_1","operation_id":"op_2","status":"removing"}`))
		default:
			writer.WriteHeader(http.StatusNotFound)
			_, _ = writer.Write([]byte(`{"detail":"Solution not found"}`))
		}
	}))
	defer server.Close()

	apiClient := New("token", server.URL, "test")
	requestContext := context.Background()

	catalog, catalogError := apiClient.GetSolutionCatalog(requestContext)
	if catalogError != nil || catalog.Plan != "launch" || len(catalog.Solutions) != 1 || catalog.Solutions[0].DefaultPreset != "small" {
		t.Fatalf("GetSolutionCatalog = %+v, %v", catalog, catalogError)
	}

	listResponse, listError := apiClient.ListSolutions(requestContext)
	if listError != nil || len(listResponse.Solutions) != 1 {
		t.Fatalf("ListSolutions = %+v, %v", listResponse, listError)
	}
	solution := listResponse.Solutions[0]
	if solution.App == nil || solution.App.Name != "web" || solution.Bindings[0].Variables[0] != "DATABASE_URL" {
		t.Fatalf("decoded solution = %+v", solution)
	}

	installResponse, installError := apiClient.InstallSolution(requestContext, InstallSolutionRequest{SolutionKey: "postgres", Preset: "small", AppID: "app_1"})
	if installError != nil || installResponse.ID != "sol_2" || len(installResponse.Required) != 1 {
		t.Fatalf("InstallSolution = %+v, %v", installResponse, installError)
	}
	if installBody.SolutionKey != "postgres" || installBody.Preset != "small" || installBody.AppID != "app_1" {
		t.Fatalf("install request body = %+v", installBody)
	}

	statusResponse, statusError := apiClient.GetSolutionStatus(requestContext, "sol_1")
	if statusError != nil || !statusResponse.Healthy || statusResponse.Checks[0].Message != "Running." {
		t.Fatalf("GetSolutionStatus = %+v, %v", statusResponse, statusError)
	}

	binding, bindError := apiClient.BindSolution(requestContext, "sol_1", "app_2")
	if bindError != nil || binding.AppName != "api" || bindBody["app_id"] != "app_2" {
		t.Fatalf("BindSolution = %+v, %v (body %v)", binding, bindError, bindBody)
	}

	if _, removeError := apiClient.RemoveSolution(requestContext, "sol_1", true); removeError != nil || removePath != "/api/v1/solutions/sol_1?force=true" {
		t.Fatalf("RemoveSolution path = %q, %v", removePath, removeError)
	}
	if _, removeError := apiClient.RemoveSolution(requestContext, "sol_1", false); removeError != nil || removePath != "/api/v1/solutions/sol_1" {
		t.Fatalf("RemoveSolution path = %q, %v", removePath, removeError)
	}
}

// The API's refusals carry a sentence for the person; the client hands it
// through untouched, with the status, so the CLI prints exactly that.
func TestInstallSolutionRefusalsKeepTheSentence(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(http.StatusPaymentRequired)
		_, _ = writer.Write([]byte(`{"detail":"Tael Managed Monitoring is available on Launch.","code":"plan_required"}`))
	}))
	defer server.Close()

	apiClient := New("token", server.URL, "test")
	_, installError := apiClient.InstallSolution(context.Background(), InstallSolutionRequest{SolutionKey: "monitoring"})
	var apiError *APIError
	if !errors.As(installError, &apiError) {
		t.Fatalf("InstallSolution error = %v, want *APIError", installError)
	}
	if apiError.StatusCode != http.StatusPaymentRequired || apiError.Detail != "Tael Managed Monitoring is available on Launch." {
		t.Fatalf("APIError = %+v", apiError)
	}
}

func TestMatchesSolution(t *testing.T) {
	solution := Solution{ID: "sol_1", Instance: "tael-postgres-web", Name: "Tael Managed Postgres for web"}
	for _, word := range []string{"sol_1", "tael-postgres-web", "Tael Managed Postgres for web", "tael managed postgres for WEB"} {
		if !MatchesSolution(solution, word) {
			t.Errorf("MatchesSolution(%q) = false, want true", word)
		}
	}
	for _, word := range []string{"", "postgres", "sol_2", "tael-postgres"} {
		if MatchesSolution(solution, word) {
			t.Errorf("MatchesSolution(%q) = true, want false", word)
		}
	}
}
