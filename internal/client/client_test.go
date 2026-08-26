package client

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDoJSONSendsAuthAndUserAgent(t *testing.T) {
	var capturedAuthorization, capturedUserAgent string
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		capturedAuthorization = request.Header.Get("Authorization")
		capturedUserAgent = request.Header.Get("User-Agent")
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"apps":[{"id":"app_1","name":"web"}]}`))
	}))
	defer server.Close()

	apiClient := New("secret-token", server.URL, "1.2.3")
	listResponse, listError := apiClient.ListApps(context.Background())
	if listError != nil {
		t.Fatalf("ListApps: %v", listError)
	}

	if capturedAuthorization != "Bearer secret-token" {
		t.Errorf("Authorization = %q, want Bearer token", capturedAuthorization)
	}
	if capturedUserAgent != "tael-cli/1.2.3" {
		t.Errorf("User-Agent = %q, want tael-cli/1.2.3", capturedUserAgent)
	}
	if len(listResponse.Apps) != 1 || listResponse.Apps[0].Name != "web" {
		t.Errorf("unexpected decoded response: %+v", listResponse)
	}
}

func TestDoJSONMapsErrorStatuses(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/api/v1/whoami":
			writer.WriteHeader(http.StatusUnauthorized)
		default:
			writer.WriteHeader(http.StatusNotFound)
			_, _ = writer.Write([]byte(`{"detail":"app not found"}`))
		}
	}))
	defer server.Close()

	apiClient := New("secret-token", server.URL, "test")

	if _, whoamiError := apiClient.Whoami(context.Background()); !errors.Is(whoamiError, ErrUnauthorized) {
		t.Errorf("Whoami error = %v, want ErrUnauthorized", whoamiError)
	}

	_, appError := apiClient.GetApp(context.Background(), "missing")
	var apiError *APIError
	if !errors.As(appError, &apiError) {
		t.Fatalf("GetApp error = %v, want *APIError", appError)
	}
	if apiError.StatusCode != http.StatusNotFound || apiError.Detail != "app not found" {
		t.Errorf("APIError = %+v, want status 404 with detail from body", apiError)
	}
}

func TestNewTrimsTrailingSlashFromBaseURL(t *testing.T) {
	apiClient := New("token", "https://api.tael.io/", "test")
	if apiClient.BaseURL != "https://api.tael.io" {
		t.Fatalf("BaseURL = %q, want trailing slash trimmed", apiClient.BaseURL)
	}
}
