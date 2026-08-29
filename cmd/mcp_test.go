package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMCPPrintsTheEndpointAndTheWiring(t *testing.T) {
	// The command never calls the API; the server only anchors the base URL.
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()

	output, runError := runCommand(t, server, "mcp")
	if runError != nil {
		t.Fatalf("tael mcp failed: %v", runError)
	}
	if !strings.Contains(output, server.URL+"/api/v1/mcp") {
		t.Fatalf("expected the endpoint under the configured base URL, got:\n%s", output)
	}
	if !strings.Contains(output, "claude mcp add tael") {
		t.Fatalf("expected a ready-to-paste client command, got:\n%s", output)
	}
	if !strings.Contains(output, "tael tokens create") {
		t.Fatalf("expected the token hint, got:\n%s", output)
	}
}

func TestMCPJSONOutput(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()

	output, runError := runCommand(t, server, "mcp", "-o", "json")
	if runError != nil {
		t.Fatalf("tael mcp -o json failed: %v", runError)
	}
	var payload struct {
		Endpoint  string `json:"endpoint"`
		Transport string `json:"transport"`
	}
	if unmarshalError := json.Unmarshal([]byte(output), &payload); unmarshalError != nil {
		t.Fatalf("decoding JSON output: %v (output %q)", unmarshalError, output)
	}
	if payload.Endpoint != server.URL+"/api/v1/mcp" || payload.Transport != "http" {
		t.Fatalf("unexpected payload: %+v", payload)
	}
}
