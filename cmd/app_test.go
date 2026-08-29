package cmd

import (
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"tael.io/cli/internal/client"
)

const appDetailJSON = `{"id":"app_1","name":"web","repo_full_name":"acme/web","status":"live","pipeline_stage":"live","live_url":"https://web.tael.site","detected_framework":"Next.js","detected_generator":"lovable","last_deploy":{"id":"dep_1","status":"succeeded","commit_sha":"1a2b3c4d5e6f","commit_message":"Ship it\n\nMore","created_at":"2026-08-29T10:00:00Z","finished_at":"2026-08-29T10:04:00Z"}}`
const appStatusJSON = `{"status":"live","live_url":"https://web.tael.site","healthy":true,"checks":[{"name":"app","status":"ok","message":"Running and healthy."},{"name":"url","status":"ok","message":"Serving at https://web.tael.site."}]}`

func TestAppCommandReadsLikeThePage(t *testing.T) {
	previousLocal := time.Local
	time.Local = time.UTC
	t.Cleanup(func() { time.Local = previousLocal })

	server, _ := newAPIServer(t,
		route{http.MethodGet, "/api/v1/apps/web", http.StatusOK, appDetailJSON},
		route{http.MethodGet, "/api/v1/apps/web/status", http.StatusOK, appStatusJSON},
	)
	output, runError := runCommand(t, server, "app", "web")
	if runError != nil {
		t.Fatalf("tael app web: %v", runError)
	}
	mustContain(t, output,
		"App:       web\n",
		"Status:    live (healthy)\n",
		"Address:   https://web.tael.site\n",
		"Repo:      acme/web\n",
		"Framework: Next.js\n",
		"Last deploy: succeeded · 1a2b3c4 Ship it · 2026-08-29 10:04\n",
		"Checks:\n  ok       app — Running and healthy.\n",
	)
	mustSpeakTael(t, output)

	jsonOutput, jsonError := runCommand(t, server, "app", "web", "-o", "json")
	if jsonError != nil || !strings.Contains(jsonOutput, `"pipeline_stage": "live"`) || !strings.Contains(jsonOutput, `"healthy": true`) {
		t.Fatalf("tael app web -o json = %q, %v", jsonOutput, jsonError)
	}
}

func TestAppCommandRefusesAnUnknownApp(t *testing.T) {
	server, _ := newAPIServer(t,
		route{http.MethodGet, "/api/v1/apps/missing", http.StatusNotFound, `{"detail":"App not found"}`},
	)
	_, runError := runCommand(t, server, "app", "missing")
	var apiError *client.APIError
	if !errors.As(runError, &apiError) || apiError.StatusCode != http.StatusNotFound || !strings.Contains(runError.Error(), "App not found") {
		t.Fatalf("tael app missing = %v, want the API's 404 sentence", runError)
	}
	if exitCodeFor(runError) != exitError {
		t.Fatalf("exit code = %d, want %d", exitCodeFor(runError), exitError)
	}
}

func TestRemoveNeedsYesAndThenRemoves(t *testing.T) {
	server, recorded := newAPIServer(t,
		route{http.MethodDelete, "/api/v1/apps/web", http.StatusOK, `{"id":"app_1","status":"removed"}`},
	)
	_, refusal := runCommand(t, server, "remove", "web")
	if refusal == nil || exitCodeFor(refusal) != exitUsage || !strings.Contains(refusal.Error(), "--yes") {
		t.Fatalf("tael remove without --yes = %v, want a usage refusal naming --yes", refusal)
	}
	if lastRequest(recorded, http.MethodDelete, "/api/v1/apps/web") != nil {
		t.Fatalf("remove without --yes must not call the API")
	}

	output, runError := runCommand(t, server, "remove", "web", "--yes")
	if runError != nil {
		t.Fatalf("tael remove web --yes: %v", runError)
	}
	mustContain(t, output, "Removed web from Tael. The repository is untouched.\n")
	if lastRequest(recorded, http.MethodDelete, "/api/v1/apps/web") == nil {
		t.Fatalf("remove --yes did not call DELETE /api/v1/apps/web")
	}
}

func TestRemoveHandsTheRefusalThrough(t *testing.T) {
	server, _ := newAPIServer(t,
		route{http.MethodDelete, "/api/v1/apps/web", http.StatusBadGateway, `{"detail":"The app could not be removed just now; nothing was changed. Try again in a moment."}`},
	)
	_, runError := runCommand(t, server, "remove", "web", "--yes")
	if runError == nil || !strings.Contains(runError.Error(), "nothing was changed") {
		t.Fatalf("tael remove = %v, want the API's sentence", runError)
	}
}

func TestRetryAndGoLive(t *testing.T) {
	server, recorded := newAPIServer(t,
		route{http.MethodPost, "/api/v1/apps/web/retry", http.StatusAccepted, `{"id":"app_1","status":"creating"}`},
		route{http.MethodPost, "/api/v1/apps/web/go-live", http.StatusOK, `{"status":"going_live","pull_request_url":"https://github.com/acme/web/pull/1"}`},
		route{http.MethodPost, "/api/v1/apps/api/retry", http.StatusConflict, `{"detail":"this app has no failed setup to retry"}`},
	)
	output, retryError := runCommand(t, server, "retry", "web")
	if retryError != nil {
		t.Fatalf("tael retry web: %v", retryError)
	}
	mustContain(t, output, "Setting up web again from where it stopped. Follow it with `tael setup web`.\n")

	_, refusal := runCommand(t, server, "retry", "api")
	if refusal == nil || !strings.Contains(refusal.Error(), "no failed setup to retry") {
		t.Fatalf("tael retry api = %v, want the 409 sentence", refusal)
	}

	output, goLiveError := runCommand(t, server, "go-live", "web")
	if goLiveError != nil {
		t.Fatalf("tael go-live web: %v", goLiveError)
	}
	mustContain(t, output, "web is going live", "Merged: https://github.com/acme/web/pull/1\n", "tael logs web -f")
	if lastRequest(recorded, http.MethodPost, "/api/v1/apps/web/go-live") == nil {
		t.Fatalf("go-live did not POST")
	}
}

func TestSetupNarratesTheReading(t *testing.T) {
	server, _ := newAPIServer(t,
		route{http.MethodGet, "/api/v1/apps/web/setup", http.StatusOK, `{"status":"awaiting_review","state":"ready","analysis_status":"done","error_message":null,"detected_framework":"Next.js","detected_language":"TypeScript","creation_progress":[{"key":"read","status":"done","message":"Read the repository."},{"key":"write","status":"running","message":null}],"generated_files":[{"path":"Dockerfile","content":"FROM node"}],"pull_request_url":"https://github.com/acme/web/pull/1"}`},
		route{http.MethodGet, "/api/v1/apps/api/setup", http.StatusOK, `{"status":"failed","error_message":"The build did not finish.","creation_progress":[{"key":"read","status":"failed","message":"Could not read the repository."}],"generated_files":null,"pull_request_url":null}`},
		route{http.MethodGet, "/api/v1/apps/fresh/setup", http.StatusBadGateway, `{"detail":"app fresh has no application yet"}`},
	)
	output, runError := runCommand(t, server, "setup", "web")
	if runError != nil {
		t.Fatalf("tael setup web: %v", runError)
	}
	mustContain(t, output,
		"Setup for web: the setup pull request is ready.\n",
		"  ✓ Read the repository.\n",
		"Detected: Next.js (TypeScript)\n",
		"Written: Dockerfile\n",
		"Setup pull request: https://github.com/acme/web/pull/1\n→ tael go-live web\n",
	)
	mustSpeakTael(t, output)

	output, failedError := runCommand(t, server, "setup", "api")
	if failedError != nil {
		t.Fatalf("tael setup api: %v", failedError)
	}
	mustContain(t, output, "Setup for api: stopped.\n", "  ✗ Could not read the repository.\n", "Stopped: The build did not finish.\n→ tael retry api\n")

	_, freshError := runCommand(t, server, "setup", "fresh")
	if freshError == nil || !strings.Contains(freshError.Error(), "has not started reading fresh yet") {
		t.Fatalf("tael setup fresh = %v, want Tael's own sentence for a 502", freshError)
	}
}

const pipelineJSON = `{"id":"pipe_1","graph_version":2,"graph":{"schema":1,"triggers":{"push_branches":["main"]},"nodes":[{"id":"build","kind":"build","name":"Build & publish image","with":{"context":"."},"managed":true},{"id":"deploy","kind":"deploy","name":"Deploy to production","with":{"environment":"production"}},{"id":"verify","kind":"verify","name":"Verify health","with":{"path":"/"}}],"edges":[{"from":"build","to":"deploy"},{"from":"deploy","to":"verify"}]}}`

func TestPipelineShowsAndSetsASetting(t *testing.T) {
	server, recorded := newAPIServer(t,
		route{http.MethodGet, "/api/v1/apps/web/pipeline", http.StatusOK, pipelineJSON},
		route{http.MethodPut, "/api/v1/apps/web/pipeline", http.StatusOK, `{"status":"revision_queued","revision_id":"rev_1","graph_version":3,"note":"Tael is opening a pull request with this change."}`},
	)
	output, runError := runCommand(t, server, "pipeline", "web")
	if runError != nil {
		t.Fatalf("tael pipeline web: %v", runError)
	}
	mustContain(t, output, "Pipeline for web (version 2)\n", "Runs on: a push to main\n", "verify", "path=/", "deploy")
	mustSpeakTael(t, output)

	output, setError := runCommand(t, server, "pipeline", "web", "--set", "verify.path=/health", "--set", "triggers.push_branches=main,develop")
	if setError != nil {
		t.Fatalf("tael pipeline web --set: %v", setError)
	}
	mustContain(t, output, "Changed the pipeline for web (now version 3). Tael is opening a pull request with this change.\n")
	body := decodeBody(t, lastRequest(recorded, http.MethodPut, "/api/v1/apps/web/pipeline"))
	if body["graph_version"] != float64(2) {
		t.Fatalf("PUT graph_version = %v, want the version the edit was made against", body["graph_version"])
	}
	if !strings.Contains((*recorded)[len(*recorded)-1].Body, `"path":"/health"`) || !strings.Contains((*recorded)[len(*recorded)-1].Body, `"push_branches":["main","develop"]`) {
		t.Fatalf("PUT body = %s, want the edited setting and branches", (*recorded)[len(*recorded)-1].Body)
	}

	_, badSet := runCommand(t, server, "pipeline", "web", "--set", "nowhere.path=/")
	if badSet == nil || exitCodeFor(badSet) != exitUsage || !strings.Contains(badSet.Error(), "no step called") {
		t.Fatalf("tael pipeline --set on an unknown step = %v, want a usage error listing the steps", badSet)
	}
}

func TestPipelineConflictIsTheAPIsSentence(t *testing.T) {
	server, _ := newAPIServer(t,
		route{http.MethodGet, "/api/v1/apps/web/pipeline", http.StatusOK, pipelineJSON},
		route{http.MethodPut, "/api/v1/apps/web/pipeline", http.StatusConflict, `{"detail":"The pipeline changed since you loaded it — reload and reapply your edit"}`},
	)
	_, runError := runCommand(t, server, "pipeline", "web", "--set", "verify.path=/health")
	if runError == nil || !strings.Contains(runError.Error(), "changed since you loaded it") {
		t.Fatalf("tael pipeline conflict = %v", runError)
	}
}

func TestDomainsMarksTheLiveOnes(t *testing.T) {
	server, _ := newAPIServer(t,
		route{http.MethodGet, "/api/v1/apps", http.StatusOK, `{"apps":[{"id":"app_1","name":"web","status":"live","live_url":"https://web.tael.site"},{"id":"app_2","name":"api","status":"building","live_url":null}]}`},
	)
	output, runError := runCommand(t, server, "domains")
	if runError != nil {
		t.Fatalf("tael domains: %v", runError)
	}
	mustContain(t, output, "APP  ADDRESS", "web  https://web.tael.site  ● live", "api  -")

	empty, _ := newAPIServer(t, route{http.MethodGet, "/api/v1/apps", http.StatusOK, `{"apps":[]}`})
	output, emptyError := runCommand(t, empty, "domains")
	if emptyError != nil || !strings.Contains(output, "No apps yet") {
		t.Fatalf("tael domains with no apps = %q, %v", output, emptyError)
	}
}
