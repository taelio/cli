package client

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestStacksEndpoints(t *testing.T) {
	var createBody CreateStackRequest
	var patchBody PatchStackRequest
	var patchedID, deletedID string
	var movePath string
	var moveBody map[string]*string
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		switch {
		case request.Method == http.MethodGet && request.URL.Path == "/api/v1/stacks":
			_, _ = writer.Write([]byte(`{"stacks":[{"id":"st_1","name":"checkout","description":"","apps":[{"id":"app_1","name":"web","status":"live","tone":"live"},{"id":"app_2","name":"api","status":"awaiting_review","tone":"warning"}],"created_at":"2026-08-30T10:00:00Z"},{"id":"st_2","name":"billing","apps":[]}]}`))
		case request.Method == http.MethodPost && request.URL.Path == "/api/v1/stacks":
			_ = json.NewDecoder(request.Body).Decode(&createBody)
			writer.WriteHeader(http.StatusCreated)
			_, _ = writer.Write([]byte(`{"id":"st_3","name":"storefront","apps":[{"id":"app_1","name":"web"}]}`))
		case request.Method == http.MethodPatch && request.URL.Path == "/api/v1/stacks/st_1":
			patchedID = "st_1"
			_ = json.NewDecoder(request.Body).Decode(&patchBody)
			_, _ = writer.Write([]byte(`{"id":"st_1","name":"payments","apps":[]}`))
		case request.Method == http.MethodDelete && request.URL.Path == "/api/v1/stacks/st_1":
			deletedID = "st_1"
			writer.WriteHeader(http.StatusNoContent)
		case request.Method == http.MethodPut && request.URL.Path == "/api/v1/apps/app_1/stack":
			movePath = request.URL.Path
			moveBody = nil
			_ = json.NewDecoder(request.Body).Decode(&moveBody)
			writer.WriteHeader(http.StatusNoContent)
		default:
			writer.WriteHeader(http.StatusNotFound)
			_, _ = writer.Write([]byte(`{"detail":"Not found"}`))
		}
	}))
	defer server.Close()
	apiClient := New("token", server.URL, "test")
	requestContext := context.Background()

	listResponse, listError := apiClient.ListStacks(requestContext)
	if listError != nil || len(listResponse.Stacks) != 2 {
		t.Fatalf("ListStacks = %+v, %v", listResponse, listError)
	}
	checkout := listResponse.Stacks[0]
	if checkout.Name != "checkout" || len(checkout.Apps) != 2 || checkout.Apps[1].Name != "api" || checkout.Apps[1].Status != "awaiting_review" {
		t.Fatalf("checkout decoded = %+v", checkout)
	}
	if billing := listResponse.Stacks[1]; billing.Name != "billing" || len(billing.Apps) != 0 {
		t.Fatalf("billing decoded = %+v", billing)
	}

	created, createError := apiClient.CreateStack(requestContext, CreateStackRequest{Name: "storefront", AppIDs: []string{"app_1"}})
	if createError != nil || created.ID != "st_3" || len(created.Apps) != 1 {
		t.Fatalf("CreateStack = %+v, %v", created, createError)
	}
	if createBody.Name != "storefront" || len(createBody.AppIDs) != 1 || createBody.AppIDs[0] != "app_1" || createBody.Description != "" {
		t.Fatalf("create stack body = %+v", createBody)
	}

	renamed, patchError := apiClient.PatchStack(requestContext, "st_1", PatchStackRequest{Name: "payments"})
	if patchError != nil || patchedID != "st_1" || renamed.Name != "payments" || patchBody.Name != "payments" {
		t.Fatalf("PatchStack = %+v, %v (body %+v)", renamed, patchError, patchBody)
	}

	if deleteError := apiClient.DeleteStack(requestContext, "st_1"); deleteError != nil || deletedID != "st_1" {
		t.Fatalf("DeleteStack: %v (deleted %q)", deleteError, deletedID)
	}

	stackID := "st_2"
	if moveError := apiClient.MoveAppToStack(requestContext, "app_1", &stackID); moveError != nil {
		t.Fatalf("MoveAppToStack: %v", moveError)
	}
	if movePath != "/api/v1/apps/app_1/stack" || moveBody["stack_id"] == nil || *moveBody["stack_id"] != "st_2" {
		t.Fatalf("move sent %s %v", movePath, moveBody)
	}

	// Ungrouping sends an explicit null.
	if moveError := apiClient.MoveAppToStack(requestContext, "app_1", nil); moveError != nil {
		t.Fatalf("MoveAppToStack(nil): %v", moveError)
	}
	if value, present := moveBody["stack_id"]; !present || value != nil {
		t.Fatalf("ungroup body = %v, want stack_id null", moveBody)
	}
}

// The API's refusals come through as its sentences.
func TestStacksRefusals(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(http.StatusConflict)
		_, _ = writer.Write([]byte(`{"detail":"A stack called checkout already exists."}`))
	}))
	defer server.Close()
	apiClient := New("token", server.URL, "test")

	_, duplicate := apiClient.CreateStack(context.Background(), CreateStackRequest{Name: "checkout"})
	var apiError *APIError
	if !errors.As(duplicate, &apiError) || apiError.StatusCode != http.StatusConflict || apiError.Detail != "A stack called checkout already exists." {
		t.Fatalf("CreateStack on 409 = %v, want the API's sentence", duplicate)
	}
}
