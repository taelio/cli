package cmd

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"tael.io/cli/internal/client"
)

const workspacesJSON = `{"workspaces":[{"id":"ws_1","slug":"acme","name":"Acme","plan":"launch","role":"owner"},{"id":"ws_2","slug":"side","name":"Side project","plan":"free","role":"member"}],"current_id":"ws_1"}`

func TestWorkspacesMarksTheOneInUse(t *testing.T) {
	server, _ := newAPIServer(t, route{http.MethodGet, "/api/v1/workspaces", http.StatusOK, workspacesJSON})
	output, runError := runCommand(t, server, "workspaces")
	if runError != nil {
		t.Fatalf("tael workspaces: %v", runError)
	}
	mustContain(t, output, "SLUG  NAME          PLAN    ROLE    ACTING\n", "acme  Acme          launch  owner   ● here\n", "side  Side project  free    member  -\n", "tael workspace use <slug>")
	mustSpeakTael(t, output)
}

func TestWorkspaceUseKeepsOnlyAChoiceThatTook(t *testing.T) {
	server, recorded := newAPIServer(t,
		route{http.MethodGet, "/api/v1/workspaces", http.StatusOK, workspacesJSON},
		route{http.MethodGet, "/api/v1/whoami", http.StatusOK, `{"user":{"id":"u_1","github_login":"dana","name":"Dana"},"workspace":{"id":"ws_1","slug":"acme","name":"Acme","plan":"launch"}}`},
	)
	output, sameError := runCommand(t, server, "workspace", "use", "acme")
	if sameError != nil {
		t.Fatalf("tael workspace use acme: %v", sameError)
	}
	mustContain(t, output, "Acting in Acme (acme): the workspace your token was made in.\n")

	_, otherError := runCommand(t, server, "workspace", "use", "side")
	if otherError == nil || !strings.Contains(otherError.Error(), "your token was made in Acme (acme) and cannot act in side") {
		t.Fatalf("tael workspace use side against a token that cannot move = %v", otherError)
	}
	whoami := lastRequest(recorded, http.MethodGet, "/api/v1/whoami")
	if whoami == nil {
		t.Fatalf("workspace use did not ask the API which workspace the token acts in")
	}
	if saved := readConfigFile(); saved.WorkspaceID != "" {
		t.Fatalf("a choice that did not take was saved: %+v", saved)
	}

	_, unknown := runCommand(t, server, "workspace", "use", "nowhere")
	if unknown == nil || exitCodeFor(unknown) != exitUsage || !strings.Contains(unknown.Error(), "yours: acme, side") {
		t.Fatalf("tael workspace use nowhere = %v, want a usage error listing the workspaces", unknown)
	}
}

func TestWorkspaceUseKeepsAChoiceThatTookAndSendsTheHeader(t *testing.T) {
	server, recorded := newAPIServer(t,
		route{http.MethodGet, "/api/v1/workspaces", http.StatusOK, workspacesJSON},
		route{http.MethodGet, "/api/v1/whoami", http.StatusOK, `{"user":{"id":"u_1"},"workspace":{"id":"ws_2","slug":"side","name":"Side project","plan":"free"}}`},
	)
	output, runError := runCommand(t, server, "workspace", "use", "side")
	if runError != nil {
		t.Fatalf("tael workspace use side: %v", runError)
	}
	mustContain(t, output, "Now acting in Side project (side).\n")
	whoami := lastRequest(recorded, http.MethodGet, "/api/v1/whoami")
	if whoami == nil || whoami.Headers.Get(client.WorkspaceHeader) != "ws_2" {
		t.Fatalf("whoami was asked without the workspace header: %+v", whoami)
	}
	saved := readConfigFile()
	if saved.WorkspaceID != "ws_2" || saved.Workspace != "side" {
		t.Fatalf("saved config = %+v, want the chosen workspace", saved)
	}
	if content, readError := os.ReadFile(filepath.Clean(os.Getenv("TAEL_CONFIG"))); readError != nil || !strings.Contains(string(content), "workspace_id: ws_2") {
		t.Fatalf("config file = %q, %v", content, readError)
	}

	// Every later command names the chosen workspace.
	apps, _ := newAPIServer(t, route{http.MethodGet, "/api/v1/apps", http.StatusOK, `{"apps":[]}`})
	configPath := os.Getenv("TAEL_CONFIG")
	t.Setenv("TAEL_CONFIG", configPath)
	if _, appsError := runCommandKeepingConfig(t, apps, configPath, "apps"); appsError != nil {
		t.Fatalf("tael apps after workspace use: %v", appsError)
	}
}
