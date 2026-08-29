package cmd

import (
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestTokensListCreateRevoke(t *testing.T) {
	previousLocal := time.Local
	time.Local = time.UTC
	t.Cleanup(func() { time.Local = previousLocal })

	server, recorded := newAPIServer(t,
		route{http.MethodGet, "/api/v1/account/tokens", http.StatusOK, `{"tokens":[{"id":"tok_1","name":"tael-cli-laptop","scopes":null,"created_at":"2026-08-01T10:00:00Z","expires_at":"2099-10-30T10:00:00Z","revoked_at":null,"last_used_at":"2026-08-29T09:00:00Z"},{"id":"tok_2","name":"ci","scopes":null,"created_at":"2026-07-01T10:00:00Z","expires_at":"2026-07-31T10:00:00Z","revoked_at":null,"last_used_at":null},{"id":"tok_3","name":"old","scopes":null,"created_at":"2026-06-01T10:00:00Z","expires_at":null,"revoked_at":"2026-06-02T10:00:00Z","last_used_at":null}]}`},
		route{http.MethodPost, "/api/v1/account/tokens", http.StatusCreated, `{"id":"tok_4","name":"deploy-bot","token":"tael_s3cr3t","expires_at":"2026-11-27T10:00:00Z"}`},
		route{http.MethodPost, "/api/v1/account/tokens/tok_1/revoke", http.StatusOK, `{"status":"revoked"}`},
		route{http.MethodPost, "/api/v1/account/tokens/tok_3/revoke", http.StatusNotFound, `{"detail":"Token not found or already revoked"}`},
	)
	output, listError := runCommand(t, server, "tokens")
	if listError != nil {
		t.Fatalf("tael tokens: %v", listError)
	}
	mustContain(t, output,
		"NAME             ID     STATE    MADE              EXPIRES           LAST USED\n",
		"tael-cli-laptop  tok_1  working  2026-08-01 10:00  2099-10-30 10:00  2026-08-29 09:00\n",
		"ci               tok_2  expired  2026-07-01 10:00  2026-07-31 10:00  -\n",
		"old              tok_3  revoked  2026-06-01 10:00  -                 -\n",
	)
	if strings.Contains(output, "tael_") {
		t.Fatalf("the list must never show a secret:\n%s", output)
	}

	output, createError := runCommand(t, server, "tokens", "create", "deploy-bot", "--expires", "90d")
	if createError != nil {
		t.Fatalf("tael tokens create: %v", createError)
	}
	mustContain(t, output, "Token made: deploy-bot\n\n  tael_s3cr3t\n\n", "This is the only time it is shown. Use it with --token or TAEL_API_TOKEN; it expires 2026-11-27 10:00.\n")
	body := decodeBody(t, lastRequest(recorded, http.MethodPost, "/api/v1/account/tokens"))
	expiresAt, _ := body["expires_at"].(string)
	if body["name"] != "deploy-bot" || !strings.HasPrefix(expiresAt, time.Now().Add(90*24*time.Hour).UTC().Format("2006-01-02")) {
		t.Fatalf("create body = %v", body)
	}
	if _, badExpiry := runCommand(t, server, "tokens", "create", "x", "--expires", "soon"); badExpiry == nil || exitCodeFor(badExpiry) != exitUsage {
		t.Fatalf("--expires soon = %v, want a usage error", badExpiry)
	}

	output, revokeError := runCommand(t, server, "tokens", "revoke", "tael-cli-laptop")
	if revokeError != nil || !strings.Contains(output, "Revoked tael-cli-laptop.") {
		t.Fatalf("tael tokens revoke by name = %q, %v", output, revokeError)
	}
	if lastRequest(recorded, http.MethodPost, "/api/v1/account/tokens/tok_1/revoke") == nil {
		t.Fatalf("revoke did not POST for the resolved token")
	}
	_, gone := runCommand(t, server, "tokens", "revoke", "tok_3")
	if gone == nil || !strings.Contains(gone.Error(), "already revoked") {
		t.Fatalf("revoking a revoked token = %v", gone)
	}
	_, unknown := runCommand(t, server, "tokens", "revoke", "nothing")
	if unknown == nil || exitCodeFor(unknown) != exitUsage {
		t.Fatalf("revoking an unknown token = %v, want a usage error", unknown)
	}
}
