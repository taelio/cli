package cmd

import (
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestTasksNewAsksTael(t *testing.T) {
	server, recorded := newAPIServer(t,
		route{http.MethodPost, "/api/v1/tasks", http.StatusCreated, `{"task":{"id":"t_1","kind":"investigate","title":"Why is checkout slow?","status":"running","created_at":"2026-08-29T10:00:00Z"},"plan":[],"changes":[],"evidence":[],"approvals":[],"comments":[]}`},
	)
	output, runError := runCommand(t, server, "tasks", "new", "why is checkout slow?", "--app", "web", "--kind", "investigate")
	if runError != nil {
		t.Fatalf("tael tasks new: %v", runError)
	}
	mustContain(t, output, "Asked Tael: Why is checkout slow?\nTael is on it (running). Follow it with `tael task t_1`.\n")
	body := decodeBody(t, lastRequest(recorded, http.MethodPost, "/api/v1/tasks"))
	if body["brief"] != "why is checkout slow?" || body["app"] != "web" || body["kind"] != "investigate" {
		t.Fatalf("tasks new body = %v", body)
	}

	proposed, _ := newAPIServer(t, route{http.MethodPost, "/api/v1/tasks", http.StatusCreated, `{"task":{"id":"t_2","title":"Fix it","status":"proposed","created_at":"2026-08-29T10:00:00Z"}}`})
	output, proposedError := runCommand(t, proposed, "tasks", "new", "fix it")
	if proposedError != nil || !strings.Contains(output, "the task is on the record: tael task t_2") {
		t.Fatalf("tael tasks new on a free workspace = %q, %v", output, proposedError)
	}

	refused, _ := newAPIServer(t, route{http.MethodPost, "/api/v1/tasks", http.StatusNotFound, `{"detail":"App not found"}`})
	_, refusal := runCommand(t, refused, "tasks", "new", "look", "--app", "nope")
	if refusal == nil || !strings.Contains(refusal.Error(), "App not found") {
		t.Fatalf("tael tasks new --app nope = %v", refusal)
	}
	_, blank := runCommand(t, refused, "tasks", "new", "   ")
	if blank == nil || exitCodeFor(blank) != exitUsage {
		t.Fatalf("tael tasks new with nothing to say = %v, want a usage error", blank)
	}
}

func TestTaskCommentAndTaskStillReadable(t *testing.T) {
	server, recorded := newAPIServer(t,
		route{http.MethodPost, "/api/v1/tasks/t_1/comments", http.StatusCreated, `{"comment":{"id":"c_1","author":"you","author_name":null,"body":"Go ahead.","created_at":"2026-08-29T10:00:00Z"}}`},
		route{http.MethodPost, "/api/v1/tasks/t_9/comments", http.StatusNotFound, `{"detail":"No such task."}`},
		route{http.MethodGet, "/api/v1/tasks/t_1", http.StatusOK, `{"task":{"id":"t_1","kind":"deploy","title":"Deploy web","status":"done","created_at":"2026-08-29T10:00:00Z"}}`},
	)
	output, runError := runCommand(t, server, "task", "comment", "t_1", "Go ahead.")
	if runError != nil {
		t.Fatalf("tael task comment: %v", runError)
	}
	mustContain(t, output, "Noted on the task: Go ahead.\n")
	if body := decodeBody(t, lastRequest(recorded, http.MethodPost, "/api/v1/tasks/t_1/comments")); body["body"] != "Go ahead." {
		t.Fatalf("comment body = %v", body)
	}
	_, missing := runCommand(t, server, "task", "comment", "t_9", "hello")
	if missing == nil || !strings.Contains(missing.Error(), "No such task.") {
		t.Fatalf("commenting on an unknown task = %v", missing)
	}
	output, taskError := runCommand(t, server, "task", "t_1")
	if taskError != nil || !strings.HasPrefix(output, "Deploy web\n") {
		t.Fatalf("tael task t_1 still reads the task: %q, %v", output, taskError)
	}
}

func TestNeedsYouReadsLikeTheStrip(t *testing.T) {
	previousLocal := time.Local
	time.Local = time.UTC
	t.Cleanup(func() { time.Local = previousLocal })

	server, _ := newAPIServer(t,
		route{http.MethodGet, "/api/v1/needs-you", http.StatusOK, `{"items":[{"task":{"id":"t_1","kind":"investigate","title":"Why did api fail?","status":"needs_approval","app":{"id":"app_2","name":"api"},"created_at":"2026-08-29T09:00:00Z"},"approval":{"id":"ap_1","task_id":"t_1","category":"rollback","risk":"medium","reversible":true,"summary":"Roll back to the previous image.","status":"pending","requested_at":"2026-08-29T10:05:00Z"}},{"task":{"id":"t_2","kind":"fix","title":"Restart web nightly","status":"proposed","created_at":"2026-08-29T08:00:00Z"},"approval":null}],"count":2}`},
	)
	output, runError := runCommand(t, server, "needs-you")
	if runError != nil {
		t.Fatalf("tael needs-you: %v", runError)
	}
	mustContain(t, output,
		"Needs you (2):\n",
		"  Why did api fail? — Roll back to the previous image. (medium risk, reversible) · api · since 2026-08-29 10:05\n",
		"  → tael approve t_1   or   tael decline t_1\n",
		"  Restart web nightly — Tael proposes this · since 2026-08-29 08:00\n",
		"  → tael approve t_2   or   tael decline t_2\n",
	)
	mustSpeakTael(t, output)

	quiet, _ := newAPIServer(t, route{http.MethodGet, "/api/v1/needs-you", http.StatusOK, `{"items":[],"count":0}`})
	output, quietError := runCommand(t, quiet, "needs-you")
	if quietError != nil || !strings.Contains(output, "Nothing needs you.") {
		t.Fatalf("tael needs-you with nothing = %q, %v", output, quietError)
	}
	refused, _ := newAPIServer(t, route{http.MethodGet, "/api/v1/needs-you", http.StatusInternalServerError, `{"detail":"Internal Server Error"}`})
	if _, refusal := runCommand(t, refused, "needs-you"); refusal == nil || exitCodeFor(refusal) != exitError {
		t.Fatalf("tael needs-you on a failure = %v", refusal)
	}
}
