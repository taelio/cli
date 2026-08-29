package cmd

import (
	"net/http"
	"regexp"
	"strings"
	"testing"
	"time"

	"tael.io/cli/internal/client"
)

const feedEventsJSON = `{"events":[{"id":1,"event_type":"step_started","payload":{"name":"app_watch_generation"},"created_at":"2026-08-29T10:00:00Z"},{"id":2,"event_type":"task_updated","payload":{"task_id":"t_1","title":"Deploy web","status":"failed"},"created_at":"2026-08-29T10:02:00Z"},{"id":3,"event_type":"tael_suggestion","payload":{"message":"web restarts every night.","offer":"I can look into it."},"created_at":"2026-08-29T10:03:00Z"},{"id":4,"event_type":"operation_created","payload":{"name":"app_onboard"},"created_at":"2026-08-29T10:04:00Z"}]}`

func TestFeedNarratesTheRecentLines(t *testing.T) {
	previousLocal := time.Local
	time.Local = time.UTC
	t.Cleanup(func() { time.Local = previousLocal })

	server, recorded := newAPIServer(t, route{http.MethodGet, "/api/v1/events", http.StatusOK, feedEventsJSON})
	output, runError := runCommand(t, server, "feed", "--last", "2")
	if runError != nil {
		t.Fatalf("tael feed: %v", runError)
	}
	if strings.Contains(output, "Reading your repo") {
		t.Fatalf("--last 2 must keep only the newest lines:\n%s", output)
	}
	mustContain(t, output, "2026-08-29 10:02 ✗ Couldn't finish: Deploy web\n", "2026-08-29 10:03   web restarts every night. I can look into it.\n")
	if strings.Contains(output, "10:04") {
		t.Fatalf("the envelope around steps must not be narrated:\n%s", output)
	}
	mustSpeakTael(t, output)
	if request := lastRequest(recorded, http.MethodGet, "/api/v1/events"); request == nil || strings.Contains(request.Path, "stream=true") {
		t.Fatalf("feed without --follow must not stream: %+v", request)
	}

	jsonOutput, jsonError := runCommand(t, server, "feed", "-o", "json")
	if jsonError != nil || !strings.Contains(jsonOutput, `"event_type": "tael_suggestion"`) {
		t.Fatalf("tael feed -o json = %q, %v", jsonOutput, jsonError)
	}

	quiet, _ := newAPIServer(t, route{http.MethodGet, "/api/v1/events", http.StatusOK, `{"events":[]}`})
	output, quietError := runCommand(t, quiet, "feed")
	if quietError != nil || !strings.Contains(output, "Quiet.") {
		t.Fatalf("tael feed with nothing = %q, %v", output, quietError)
	}
}

func TestFeedFollowResumesAfterTheNewestLine(t *testing.T) {
	previousLocal := time.Local
	time.Local = time.UTC
	t.Cleanup(func() { time.Local = previousLocal })

	var streamRequests []string
	server := newStreamServer(t, func(request *http.Request) (int, string) {
		if request.URL.Query().Get("stream") == "true" {
			streamRequests = append(streamRequests, request.URL.RequestURI())
			return http.StatusOK, "id: 5\ndata: {\"id\":5,\"event_type\":\"step_completed\",\"payload\":{\"name\":\"app_watch_installation\"},\"created_at\":\"2026-08-29T10:05:00Z\"}\n\n"
		}
		return http.StatusOK, feedEventsJSON
	})
	output, runError := runCommand(t, server, "feed", "--follow", "--last", "1")
	if runError != nil {
		t.Fatalf("tael feed --follow: %v", runError)
	}
	mustContain(t, output, "2026-08-29 10:03   web restarts every night. I can look into it.\n", "2026-08-29 10:05   Your app is running.\n")
	if len(streamRequests) != 1 || !strings.Contains(streamRequests[0], "since=4") {
		t.Fatalf("stream requests = %v, want one resuming after the newest event seen", streamRequests)
	}

	refused, _ := newAPIServer(t, route{http.MethodGet, "/api/v1/events", http.StatusForbidden, `{"detail":"Not a member of the requested workspace"}`})
	if _, refusal := runCommand(t, refused, "feed"); refusal == nil || exitCodeFor(refusal) != exitAuth {
		t.Fatalf("tael feed refused = %v, want the auth exit code", refusal)
	}
}

func TestNarrateFeedEventSpeaksTael(t *testing.T) {
	identifier := regexp.MustCompile(`^[a-z0-9]+(?:[_-][a-z0-9]+)+$`)
	cases := []struct {
		name        string
		event       client.Event
		wantLine    string
		wantFailure bool
	}{
		{"a known step", event("step_started", map[string]any{"name": "workspace_create_ankra_organisation"}), "Reserving your own private space to run in.", false},
		{"a known step done", event("step_completed", map[string]any{"name": "solution_install"}), "It is installed and ready.", false},
		{"an unknown step", event("step_started", map[string]any{"name": "future_step_name"}), "Working on it.", false},
		{"a failed step keeps its sentence", event("step_failed", map[string]any{"name": "app_deploy", "message": "The image would not pull."}), "The image would not pull.", true},
		{"a failed step without one", event("step_failed", map[string]any{"name": "app_deploy", "error": "exit 1"}), "Something didn't work — I'm looking into it.", true},
		{"a proposal", event("task_created", map[string]any{"task_id": "t", "title": "Restart web nightly", "status": "proposed"}), "I propose: Restart web nightly", false},
		{"needs an OK", event("approval_requested", map[string]any{"task_id": "t", "title": "Fix api", "summary": "Roll back to the previous image."}), "Needs your OK: Roll back to the previous image.", false},
		{"auto approved", event("approval_decided", map[string]any{"task_id": "t", "title": "Restart web", "decision": "auto"}), "Pre-approved, so I went ahead: Restart web", false},
		{"a problem found", event("artifact_added", map[string]any{"task_id": "t", "title": "Why?", "artifact": "evidence", "artifact_title": "GET / → 503", "ok": false}), "Found a problem: GET / → 503", true},
		{"a change", event("artifact_added", map[string]any{"task_id": "t", "title": "Why?", "artifact": "change", "artifact_title": "Pin the image tag"}), "Changed something: Pin the image tag", false},
		{"reading the repo", event("app_analysis_progress", map[string]any{"app_id": "a"}), "Still reading the repo…", false},
		{"an identifier is not prose", event("step_started", map[string]any{"name": "app_probe", "message": "app_probe_v2"}), "Checking your app answers.", false},
		{"the envelope", event("operation_completed", map[string]any{}), "", false},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			line, failure := narrateFeedEvent(testCase.event)
			if line != testCase.wantLine || failure != testCase.wantFailure {
				t.Fatalf("narrateFeedEvent = (%q, %v), want (%q, %v)", line, failure, testCase.wantLine, testCase.wantFailure)
			}
			if identifier.MatchString(line) {
				t.Fatalf("narrateFeedEvent leaked an identifier: %q", line)
			}
			mustSpeakTael(t, line)
		})
	}
	for name, phases := range feedStepNarration {
		for _, text := range []string{phases.started, phases.completed} {
			if strings.Contains(text, "_") {
				t.Errorf("feedStepNarration[%s] = %q leaks an identifier", name, text)
			}
			mustSpeakTael(t, text)
		}
	}
}
