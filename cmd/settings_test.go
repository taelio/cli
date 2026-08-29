package cmd

import (
	"net/http"
	"strings"
	"testing"
)

const aiSettingsJSON = `{"plan":"scale","paused":false,"paused_at":null,"paused_by":null,"pre_approved":{"restart":{"per_month":5},"scale":{"per_month":0}},"quiet_hours":{"start":"22:00","end":"07:00","timezone":"Europe/Dublin"},"approvers":"admins","allowance_exhausted_until":null,"budget":50,"actions_this_month":12}`

func TestSettingsAIShowsAndChanges(t *testing.T) {
	server, recorded := newAPIServer(t,
		route{http.MethodGet, "/api/v1/workspace/ai-settings", http.StatusOK, aiSettingsJSON},
		route{http.MethodPut, "/api/v1/workspace/ai-settings", http.StatusOK, strings.Replace(strings.Replace(aiSettingsJSON, `"approvers":"admins"`, `"approvers":"members"`, 1), `"restart":{"per_month":5},"scale":{"per_month":0}`, `"restart":{"per_month":5},"rollback":{"per_month":2}`, 1)},
	)
	output, showError := runCommand(t, server, "settings", "ai")
	if showError != nil {
		t.Fatalf("tael settings ai: %v", showError)
	}
	mustContain(t, output,
		"Tael on this workspace (plan: scale)\n",
		"Paused:       no\n",
		"Approvers:    owners and admins\n",
		"Pre-approved: restart up to 5 a month\n",
		"Quiet hours:  22:00–07:00 Europe/Dublin\n",
		"This month:   12 of 50 actions used\n",
	)
	mustSpeakTael(t, output)
	if lastRequest(recorded, http.MethodPut, "/api/v1/workspace/ai-settings") != nil {
		t.Fatalf("showing the settings must not change them")
	}

	output, changeError := runCommand(t, server, "settings", "ai", "--approvers", "members", "--pre-approve", "rollback=2", "--pre-approve", "scale=0", "--quiet-hours", "23:00-06:00@Europe/Dublin")
	if changeError != nil {
		t.Fatalf("tael settings ai --approvers members: %v", changeError)
	}
	mustContain(t, output, "Approvers:    every member\n", "Pre-approved: restart up to 5 a month; rollback up to 2 a month\n")
	body := decodeBody(t, lastRequest(recorded, http.MethodPut, "/api/v1/workspace/ai-settings"))
	if body["approvers"] != "members" || body["paused"] != nil {
		t.Fatalf("settings body = %v", body)
	}
	preApproved := body["pre_approved"].(map[string]any)
	if len(preApproved) != 2 || preApproved["restart"].(map[string]any)["per_month"] != float64(5) || preApproved["rollback"].(map[string]any)["per_month"] != float64(2) {
		t.Fatalf("pre_approved = %v, want restart kept, rollback added, scale dropped", preApproved)
	}
	quietHours := body["quiet_hours"].(map[string]any)
	if quietHours["start"] != "23:00" || quietHours["end"] != "06:00" || quietHours["timezone"] != "Europe/Dublin" {
		t.Fatalf("quiet_hours = %v", quietHours)
	}

	if _, clearError := runCommand(t, server, "settings", "ai", "--clear-quiet-hours"); clearError != nil {
		t.Fatalf("tael settings ai --clear-quiet-hours: %v", clearError)
	}
	if body := decodeBody(t, lastRequest(recorded, http.MethodPut, "/api/v1/workspace/ai-settings")); body["clear_quiet_hours"] != true || body["quiet_hours"] != nil {
		t.Fatalf("clear body = %v", body)
	}
}

func TestSettingsAIRefusals(t *testing.T) {
	server, _ := newAPIServer(t,
		route{http.MethodGet, "/api/v1/workspace/ai-settings", http.StatusOK, aiSettingsJSON},
		route{http.MethodPut, "/api/v1/workspace/ai-settings", http.StatusBadRequest, `{"detail":"\"delete\" always needs a person; it cannot be pre-approved"}`},
	)
	for _, args := range [][]string{
		{"settings", "ai", "--approvers", "everyone"},
		{"settings", "ai", "--pre-approve", "restart"},
		{"settings", "ai", "--quiet-hours", "late"},
		{"settings", "ai", "--quiet-hours", "22:00-07:00", "--clear-quiet-hours"},
	} {
		if _, usageError := runCommand(t, server, args...); usageError == nil || exitCodeFor(usageError) != exitUsage {
			t.Errorf("%v = %v, want a usage error", args, usageError)
		}
	}
	_, refusal := runCommand(t, server, "settings", "ai", "--pre-approve", "delete=1")
	if refusal == nil || !strings.Contains(refusal.Error(), "always needs a person") {
		t.Fatalf("pre-approving delete = %v, want the API's sentence", refusal)
	}
}
