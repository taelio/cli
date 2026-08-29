package cmd

import (
	"net/http"
	"strings"
	"testing"
	"time"
)

const digestFactsJSON = `{"window_days":7,"from":"2026-08-22T00:00:00Z","to":"2026-08-29T00:00:00Z","deploys_total":12,"deploys_succeeded":11,"deploys_failed":1,"apps_deployed":["web","api"],"failed_deploys":[{"app":"api","when":"Thursday","error":"the container exited on boot"}],"incidents_opened":2,"incidents_resolved":1,"open_incidents":[{"app":"api","title":"api is down","severity":"high","open_for":"2 hours"}],"open_suggestions":[{"message":"web restarts every night","offer":"I can look into it."}],"needs_you":[{"task":"Roll back api","app":"api","ask":"Roll back to the previous image?","category":"rollback","waiting_for":"an hour"}],"new_members":1}`

func TestDigestReadsTheProseThenTheNumbers(t *testing.T) {
	server, recorded := newAPIServer(t,
		route{http.MethodGet, "/api/v1/digest", http.StatusOK, `{"facts":` + digestFactsJSON + `,"prose":{"headline":"A busy week; api needs you.","what_changed":"Twelve deploys, eleven clean.","what_broke":"api exited on boot on Thursday and is still down.","needs_you":["Roll back api or fix the boot error.","Someone new joined; say hello."]},"model":"m"}`},
	)
	output, runError := runCommand(t, server, "digest", "--days", "7")
	if runError != nil {
		t.Fatalf("tael digest: %v", runError)
	}
	mustContain(t, output,
		"A busy week; api needs you.\n",
		"What changed: Twelve deploys, eleven clean.\n",
		"What broke: api exited on boot on Thursday and is still down.\n",
		"Needs you:\n  - Roll back api or fix the boot error.\n  - Someone new joined; say hello.\n",
		"Numbers: 12 deploys (11 succeeded, 1 failed) across web, api · 2 incidents opened, 1 resolved · 1 still open · 1 decision on you · 1 suggestion · 1 new member · last 7 days\n",
	)
	mustSpeakTael(t, output)
	if request := lastRequest(recorded, http.MethodGet, "/api/v1/digest"); request == nil || !strings.HasSuffix(request.Path, "?days=7") {
		t.Fatalf("digest request = %+v, want ?days=7", request)
	}
}

func TestDigestWhileTaelIsStillWriting(t *testing.T) {
	server, _ := newAPIServer(t,
		route{http.MethodGet, "/api/v1/digest", http.StatusOK, `{"facts":` + digestFactsJSON + `,"prose":null,"model":"","writing":true}`},
	)
	output, runError := runCommand(t, server, "digest")
	if runError != nil {
		t.Fatalf("tael digest: %v", runError)
	}
	mustContain(t, output,
		"Tael is still writing the reading; ask again in a moment. The numbers are exact already.\n",
		"Failed deploy: api Thursday — the container exited on boot\n",
		"Open incident: api — api is down (high, open for 2 hours)\n",
		"Needs you:\n  - Roll back to the previous image? (api)\n",
		"Numbers: 12 deploys",
	)

	refused, _ := newAPIServer(t, route{http.MethodGet, "/api/v1/digest", http.StatusBadRequest, `{"detail":"days must be a positive number"}`})
	_, refusal := runCommand(t, refused, "digest", "--days", "-1")
	if refusal == nil || !strings.Contains(refusal.Error(), "days must be a positive number") {
		t.Fatalf("tael digest --days -1 = %v", refusal)
	}
}

func TestSuggestionsListAndResolve(t *testing.T) {
	previousLocal := time.Local
	time.Local = time.UTC
	t.Cleanup(func() { time.Local = previousLocal })

	server, recorded := newAPIServer(t,
		route{http.MethodGet, "/api/v1/suggestions", http.StatusOK, `{"suggestions":[{"id":"sug_1","app_id":"app_1","kind":"restart_loop","message":"web restarts every night.","offer":"I can look into it.","created_at":"2026-08-29T02:00:00Z","resolved_at":null}]}`},
		route{http.MethodGet, "/api/v1/apps", http.StatusOK, `{"apps":[{"id":"app_1","name":"web"}]}`},
		route{http.MethodPost, "/api/v1/suggestions/sug_1/resolve", http.StatusNoContent, ``},
		route{http.MethodPost, "/api/v1/suggestions/sug_9/resolve", http.StatusNotFound, `{"detail":"No such suggestion."}`},
	)
	output, runError := runCommand(t, server, "suggestions")
	if runError != nil {
		t.Fatalf("tael suggestions: %v", runError)
	}
	mustContain(t, output, "ID     APP  TAEL NOTICED               OFFERS               WHEN\n", "sug_1  web  web restarts every night.  I can look into it.  2026-08-29 02:00\n")
	mustSpeakTael(t, output)

	if _, allError := runCommand(t, server, "suggestions", "--all"); allError != nil {
		t.Fatalf("tael suggestions --all: %v", allError)
	}
	if request := lastRequest(recorded, http.MethodGet, "/api/v1/suggestions"); request == nil || !strings.Contains(request.Path, "include_resolved=true") {
		t.Fatalf("--all did not ask for resolved ones: %+v", request)
	}

	output, resolveError := runCommand(t, server, "suggestions", "resolve", "sug_1")
	if resolveError != nil || !strings.Contains(output, "Marked as dealt with.") {
		t.Fatalf("tael suggestions resolve = %q, %v", output, resolveError)
	}
	_, missing := runCommand(t, server, "suggestions", "resolve", "sug_9")
	if missing == nil || !strings.Contains(missing.Error(), "No such suggestion.") {
		t.Fatalf("resolving an unknown suggestion = %v", missing)
	}

	empty, _ := newAPIServer(t, route{http.MethodGet, "/api/v1/suggestions", http.StatusOK, `{"suggestions":[]}`})
	output, emptyError := runCommand(t, empty, "suggestions")
	if emptyError != nil || !strings.Contains(output, "Nothing to suggest right now.") {
		t.Fatalf("tael suggestions with none = %q, %v", output, emptyError)
	}
}
