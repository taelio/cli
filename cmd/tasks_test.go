package cmd

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"testing"
	"time"

	"tael.io/cli/internal/client"
)

func stringPtr(value string) *string { return &value }
func boolPtr(value bool) *bool       { return &value }

func TestRenderTasksTable(t *testing.T) {
	previousLocal := time.Local
	time.Local = time.UTC
	t.Cleanup(func() { time.Local = previousLocal })

	tasks := []client.Task{
		{
			ID:        "6f9c2a3e-0b7d-4c1e-9a55-2d8f1e7b4c10",
			Kind:      "investigate",
			Title:     "Why did website-demo fail to deploy?",
			Status:    "needs_approval",
			NeedsYou:  true,
			App:       &client.TaskApp{ID: "app-1", Name: "website-demo"},
			CreatedAt: "2026-08-29T10:00:00Z",
		},
		{
			ID:         "0d1e2f3a-4b5c-4d6e-8f70-1a2b3c4d5e6f",
			Kind:       "deploy",
			Title:      "Deploy sha-1a2b3c",
			Status:     "done",
			CreatedAt:  "2026-08-28T09:00:00Z",
			FinishedAt: stringPtr("2026-08-28T09:04:00Z"),
		},
	}

	rendered := renderTasksTable(tasks)

	expected := "" +
		fmt.Sprintf("%-38s%-11s%-15s%-38s%-14s%s\n", "ID", "STATUS", "KIND", "TITLE", "APP", "WHEN") +
		fmt.Sprintf("%-38s%-11s%-15s%-38s%-14s%s\n", tasks[0].ID, "needs you", "investigation", "Why did website-demo fail to deploy?", "website-demo", "2026-08-29 10:00") +
		fmt.Sprintf("%-38s%-11s%-15s%-38s%-14s%s\n", tasks[1].ID, "done", "deploy", "Deploy sha-1a2b3c", "-", "2026-08-28 09:04")

	if rendered != expected {
		t.Fatalf("renderTasksTable mismatch\ngot:\n%s\nwant:\n%s", rendered, expected)
	}
}

func TestRenderTaskDetailReadsLikeThePage(t *testing.T) {
	previousLocal := time.Local
	time.Local = time.UTC
	t.Cleanup(func() { time.Local = previousLocal })

	detail := &client.TaskDetail{
		Task: client.Task{
			ID:        "6f9c2a3e-0b7d-4c1e-9a55-2d8f1e7b4c10",
			Kind:      "investigate",
			Title:     "Why did website-demo fail to deploy?",
			Brief:     "The deploy failed twice in a row.",
			Status:    "needs_approval",
			App:       &client.TaskApp{ID: "app-1", Name: "website-demo"},
			CreatedAt: "2026-08-29T10:00:00Z",
		},
		Plan: []client.PlanStep{
			{ID: "st-1", Position: 1, Title: "Gather the facts", Status: "done"},
			{ID: "st-2", Position: 2, Title: "Roll back to the previous image", Detail: "The new image exits on boot.", Risk: "medium", Reversible: true, NeedsApproval: true, Status: "waiting_approval"},
			{ID: "st-3", Position: 3, Title: "Check it took", Status: "pending"},
		},
		Changes: []client.Artifact{
			{ID: "ch-1", Kind: "change", Subkind: "pull_request", Title: "Pin the image tag", URL: stringPtr("https://github.com/acme/web/pull/7")},
		},
		Evidence: []client.Artifact{
			{ID: "ev-1", Kind: "evidence", Subkind: "check", Title: "Installation healthy", OK: boolPtr(false), Body: "1 of 2 pods not ready"},
			{ID: "ev-2", Kind: "evidence", Subkind: "transcript", Title: "What I looked at", Body: "- Looked at the pods → 1 of 2 not ready\n- Read the last 300 log lines\n"},
			{ID: "ev-3", Kind: "evidence", Subkind: "probe", Title: "GET https://web.tael.site/ → 503 in 1200 ms", OK: boolPtr(false)},
		},
		Outcome: &client.TaskOutcome{Summary: "The new image exits on boot because DATABASE_URL is unset.", Next: "Roll back, then add the variable."},
		Approvals: []client.Approval{
			{ID: "ap-1", TaskID: "6f9c2a3e-0b7d-4c1e-9a55-2d8f1e7b4c10", StepID: stringPtr("st-2"), Category: "rollback", Risk: "medium", Reversible: true, Summary: "Roll back to the previous image.", Status: "pending", RequestedAt: "2026-08-29T10:05:00Z"},
		},
		Comments: []client.TaskComment{
			{ID: "c-1", Author: "tael", Body: "I can undo this if it does not help.", CreatedAt: "2026-08-29T10:06:00Z"},
			{ID: "c-2", Author: "member", AuthorName: stringPtr("Dana"), Body: "Go ahead.", CreatedAt: "2026-08-29T10:07:00Z"},
		},
	}

	var builder strings.Builder
	renderTaskDetail(&builder, detail)
	rendered := builder.String()

	for _, want := range []string{
		"Why did website-demo fail to deploy?\n",
		"Status: needs you · Kind: investigation · App: website-demo · Opened: 2026-08-29 10:00\n",
		"Why: The deploy failed twice in a row.\n",
		"\nPlan\n",
		"  1. [done] Gather the facts\n",
		"  2. [waiting for you] Roll back to the previous image (medium risk, reversible)\n",
		"     The new image exits on boot.\n",
		"     Needs your OK: Roll back to the previous image.\n",
		"     → tael approve 6f9c2a3e-0b7d-4c1e-9a55-2d8f1e7b4c10   or   tael decline 6f9c2a3e-0b7d-4c1e-9a55-2d8f1e7b4c10\n",
		"  3. [pending] Check it took\n",
		"\nChange\n  pull request: Pin the image tag  https://github.com/acme/web/pull/7\n",
		"\nEvidence\n",
		"  ✗ Installation healthy\n      1 of 2 pods not ready\n",
		"  What I looked at:\n    - Looked at the pods → 1 of 2 not ready\n    - Read the last 300 log lines\n",
		"  ✗ GET https://web.tael.site/ → 503 in 1200 ms\n",
		"\nOutcome\n  The new image exits on boot because DATABASE_URL is unset.\n  Next: Roll back, then add the variable.\n",
		"\nComments\n  Tael (2026-08-29 10:06): I can undo this if it does not help.\n  Dana (2026-08-29 10:07): Go ahead.\n",
	} {
		if !strings.Contains(rendered, want) {
			t.Errorf("renderTaskDetail is missing %q\ngot:\n%s", want, rendered)
		}
	}
	if strings.Contains(strings.ToLower(rendered), "ankra") {
		t.Errorf("renderTaskDetail names the platform:\n%s", rendered)
	}
}

func TestRenderWhyAnswersFromTheOutcome(t *testing.T) {
	previousLocal := time.Local
	time.Local = time.UTC
	t.Cleanup(func() { time.Local = previousLocal })

	task := client.Task{
		ID:         "6f9c2a3e-0b7d-4c1e-9a55-2d8f1e7b4c10",
		Title:      "Why did website-demo fail to deploy?",
		Status:     "failed",
		CreatedAt:  "2026-08-29T10:00:00Z",
		FinishedAt: stringPtr("2026-08-29T10:20:00Z"),
		Outcome:    &client.TaskOutcome{Summary: "The container exits on boot: DATABASE_URL is unset.", Next: "I'd add the variable and redeploy."},
	}
	expected := "Why did website-demo fail to deploy? (failed, 2026-08-29 10:20)\n" +
		"The container exits on boot: DATABASE_URL is unset.\n" +
		"Next: I'd add the variable and redeploy.\n" +
		"Evidence: tael task 6f9c2a3e-0b7d-4c1e-9a55-2d8f1e7b4c10\n"
	if rendered := renderWhy(task); rendered != expected {
		t.Fatalf("renderWhy mismatch\ngot:\n%s\nwant:\n%s", rendered, expected)
	}

	bare := client.Task{ID: "t", Title: "T", Status: "running", CreatedAt: "2026-08-29T10:00:00Z"}
	if rendered := renderWhy(bare); !strings.Contains(rendered, "has not written up what happened yet") {
		t.Fatalf("renderWhy without an outcome = %q", rendered)
	}

	tasks := []client.Task{{ID: "a", Status: "done"}, {ID: "b", Status: "failed"}, {ID: "c", Status: "failed"}}
	if failed := newestFailedTask(tasks); failed == nil || failed.ID != "b" {
		t.Fatalf("newestFailedTask = %v, want the first failed one", failed)
	}
	if failed := newestFailedTask([]client.Task{{ID: "a", Status: "done"}}); failed != nil {
		t.Fatalf("newestFailedTask = %v, want nil", failed)
	}
}

func event(eventType string, payload map[string]any) client.Event {
	encoded, _ := json.Marshal(payload)
	return client.Event{ID: 1, EventType: eventType, Payload: encoded, CreatedAt: "2026-08-29T10:00:00Z"}
}

func TestNarrateTaskEventFollowsOneTaskInTaelsWords(t *testing.T) {
	const taskID = "6f9c2a3e-0b7d-4c1e-9a55-2d8f1e7b4c10"
	identifier := regexp.MustCompile(`^[a-z0-9]+(?:[_-][a-z0-9]+)+$`)
	cases := []struct {
		name       string
		event      client.Event
		wantLine   string
		wantStatus string
		wantOK     bool
	}{
		{"created", event("task_created", map[string]any{"task_id": taskID, "status": "running"}), "Started.", "running", true},
		{"running", event("task_updated", map[string]any{"task_id": taskID, "status": "running"}), "Working on it.", "running", true},
		{"needs approval", event("approval_requested", map[string]any{"task_id": taskID, "summary": "Roll back to the previous image."}), "Needs your OK: Roll back to the previous image.", "needs_approval", true},
		{"approved", event("approval_decided", map[string]any{"task_id": taskID, "decision": "approved"}), "Approved — on it.", "", true},
		{"problem found", event("artifact_added", map[string]any{"task_id": taskID, "artifact": "evidence", "artifact_title": "GET / → 503", "ok": false}), "Found a problem: GET / → 503", "", true},
		{"noted", event("artifact_added", map[string]any{"task_id": taskID, "artifact": "evidence", "artifact_title": "Image published", "ok": true}), "Noted: Image published", "", true},
		{"done", event("task_updated", map[string]any{"task_id": taskID, "status": "done"}), "Done.", "done", true},
		{"failed", event("task_updated", map[string]any{"task_id": taskID, "status": "failed"}), "Couldn't finish.", "failed", true},
		{"a step of the lane", event("step_started", map[string]any{"name": "task_gather_evidence"}), "Gathering the facts: the deploy, the pods, the logs.", "", true},
		{"another task", event("task_updated", map[string]any{"task_id": "other", "status": "done"}), "", "", false},
		{"an unrelated step", event("step_started", map[string]any{"name": "workspace_create_ankra_organisation"}), "", "", false},
		{"a suggestion", event("tael_suggestion", map[string]any{"message": "x"}), "", "", false},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			line, status, ok := narrateTaskEvent(testCase.event, taskID)
			if ok != testCase.wantOK || line != testCase.wantLine || status != testCase.wantStatus {
				t.Fatalf("narrateTaskEvent = (%q, %q, %v), want (%q, %q, %v)", line, status, ok, testCase.wantLine, testCase.wantStatus, testCase.wantOK)
			}
			if identifier.MatchString(line) || strings.Contains(strings.ToLower(line), "ankra") {
				t.Fatalf("narrateTaskEvent leaked an identifier: %q", line)
			}
		})
	}
	for name, narration := range stepNarration {
		if identifier.MatchString(narration) || strings.Contains(narration, "_") || strings.Contains(strings.ToLower(narration), "ankra") {
			t.Errorf("stepNarration[%s] = %q leaks an identifier", name, narration)
		}
	}
	for _, status := range []string{"done", "failed", "declined", "superseded", "paused", "needs_approval"} {
		if !isTerminalTaskStatus(status) {
			t.Errorf("isTerminalTaskStatus(%s) = false", status)
		}
	}
	if isTerminalTaskStatus("running") || isTerminalTaskStatus("proposed") {
		t.Errorf("running and proposed are not terminal")
	}
}

func TestRenderDecisionAndPauseState(t *testing.T) {
	detail := &client.TaskDetail{Task: client.Task{ID: "t-1", Title: "Roll back", Status: "running"}}
	if got := renderDecision(detail, true); got != "Approved: Roll back\nTael is on it (running). Follow it with `tael task t-1`.\n" {
		t.Fatalf("renderDecision(approve) = %q", got)
	}
	declined := &client.TaskDetail{Task: client.Task{ID: "t-1", Title: "Roll back", Status: "declined"}}
	if got := renderDecision(declined, false); got != "Declined: Roll back\nTael won't do that; the task keeps the record (declined).\n" {
		t.Fatalf("renderDecision(decline) = %q", got)
	}
	if got := renderPauseState(&client.AISettings{Paused: true}); !strings.Contains(got, "Tael is paused") {
		t.Fatalf("renderPauseState(paused) = %q", got)
	}
	if got := renderPauseState(&client.AISettings{Paused: false}); !strings.Contains(got, "watching again") {
		t.Fatalf("renderPauseState(resumed) = %q", got)
	}
}
