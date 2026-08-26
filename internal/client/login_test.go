package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestLoginPollPendingThenToken verifies the two poll outcomes: a 202
// {"status":"pending"} answer while the browser login is in progress, then
// the 200 answer carrying the token.
func TestLoginPollPendingThenToken(t *testing.T) {
	pollCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/api/v1/cli/login/poll" {
			t.Errorf("unexpected path %s", request.URL.Path)
		}
		pollCount++
		writer.Header().Set("Content-Type", "application/json")
		if pollCount == 1 {
			writer.WriteHeader(http.StatusAccepted)
			_, _ = writer.Write([]byte(`{"status":"pending"}`))
			return
		}
		_, _ = writer.Write([]byte(`{"token":"tael_token_123","workspace_slug":"acme"}`))
	}))
	defer server.Close()

	apiClient := New("", server.URL, "test")
	pollRequest := LoginPollRequest{Ticket: "ticket-1", CodeVerifier: "verifier-1"}

	pending, pendingError := apiClient.LoginPoll(context.Background(), pollRequest)
	if pendingError != nil {
		t.Fatalf("pending poll: %v", pendingError)
	}
	if pending.Token != "" || pending.Status != "pending" {
		t.Errorf("pending poll = %+v, want empty token with pending status", pending)
	}

	released, releasedError := apiClient.LoginPoll(context.Background(), pollRequest)
	if releasedError != nil {
		t.Fatalf("released poll: %v", releasedError)
	}
	if released.Token != "tael_token_123" || released.WorkspaceSlug != "acme" {
		t.Errorf("released poll = %+v, want token and workspace slug", released)
	}
}
