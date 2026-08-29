package cmd

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"tael.io/cli/internal/client"
)

const reposJSON = `{"repos":[{"installation_id":42,"full_name":"acme/web","default_branch":"main","private":true,"language":"TypeScript","description":"The storefront","pushed_at":"2026-08-28T10:00:00Z","detected_generator":"lovable"},{"installation_id":42,"full_name":"acme/api","default_branch":"trunk","private":false,"language":"Go","pushed_at":"2026-08-29T10:00:00Z","detected_generator":""}]}`

func TestReposListsWhatTaelCanSee(t *testing.T) {
	previousLocal := time.Local
	time.Local = time.UTC
	t.Cleanup(func() { time.Local = previousLocal })

	server, _ := newAPIServer(t, route{http.MethodGet, "/api/v1/github/repos", http.StatusOK, reposJSON})
	output, runError := runCommand(t, server, "repos")
	if runError != nil {
		t.Fatalf("tael repos: %v", runError)
	}
	mustContain(t, output, "REPOSITORY  BRANCH  LANGUAGE    PUSHED", "acme/web    main    TypeScript  2026-08-28 10:00  private, made with lovable", "acme/api    trunk   Go", "tael new --repo owner/name")
	if strings.Index(output, "acme/web") > strings.Index(output, "acme/api") {
		t.Fatalf("repos made with a generator come first:\n%s", output)
	}
	mustSpeakTael(t, output)

	empty, _ := newAPIServer(t, route{http.MethodGet, "/api/v1/github/repos", http.StatusOK, `{"repos":[]}`})
	output, emptyError := runCommand(t, empty, "repos")
	if emptyError != nil || !strings.Contains(output, "Connecting GitHub is a browser step") {
		t.Fatalf("tael repos with nothing = %q, %v", output, emptyError)
	}
}

func TestReposHandsTheRefusalThrough(t *testing.T) {
	server, _ := newAPIServer(t, route{http.MethodGet, "/api/v1/github/repos", http.StatusBadGateway, `{"detail":"GitHub did not answer."}`})
	_, runError := runCommand(t, server, "repos")
	if runError == nil || !strings.Contains(runError.Error(), "GitHub did not answer.") {
		t.Fatalf("tael repos = %v, want the API's sentence", runError)
	}
}

func TestNewReadsSetsUpAndFollows(t *testing.T) {
	previousInterval := setupPollInterval
	setupPollInterval = 20 * time.Millisecond
	t.Cleanup(func() { setupPollInterval = previousInterval })

	server, recorded := newAPIServer(t,
		route{http.MethodGet, "/api/v1/github/repos", http.StatusOK, reposJSON},
		route{http.MethodPost, "/api/v1/repos/analyse", http.StatusOK, `{"framework":"Next.js","language":"TypeScript","summary":"A storefront with a cart.","needs_database":true,"database_reason":"It reads DATABASE_URL in prisma.schema","has_dockerfile":false,"build_strategy":"buildpack","listens_on_port":3000,"required_environment":["DATABASE_URL","STRIPE_KEY"],"concerns":["The Stripe key is read at build time"],"confidence":"high","model":"m","partial":false}`},
		route{http.MethodPost, "/api/v1/apps", http.StatusCreated, `{"id":"app_9"}`},
		route{http.MethodPost, "/api/v1/solutions", http.StatusCreated, `{"id":"sol_1","operation_id":"op_1","required":[]}`},
		route{http.MethodGet, "/api/v1/events", http.StatusOK, "id: 1\ndata: {\"id\":1,\"event_type\":\"step_started\",\"payload\":{\"name\":\"app_watch_generation\"}}\n\nid: 2\ndata: {\"id\":2,\"event_type\":\"app_analysis_progress\",\"payload\":{\"app_id\":\"app_9\",\"status\":\"analyzing\"}}\n\n"},
		route{http.MethodGet, "/api/v1/apps/app_9/setup", http.StatusOK, `{"status":"awaiting_review","creation_progress":[{"key":"read","status":"done","message":"Read the repository."},{"key":"write","status":"done","message":"Wrote the setup."}],"generated_files":[],"pull_request_url":"https://github.com/acme/web/pull/3"}`},
		route{http.MethodPost, "/api/v1/apps/app_9/go-live", http.StatusOK, `{"status":"going_live","pull_request_url":"https://github.com/acme/web/pull/3"}`},
	)
	output, runError := runCommand(t, server, "new", "--repo", "ACME/web", "--database", "--go-live")
	if runError != nil {
		t.Fatalf("tael new: %v\n%s", runError, output)
	}
	mustContain(t, output,
		"Reading acme/web…\n",
		"This looks like Next.js. A storefront with a cart.\n",
		"It wants a database: It reads DATABASE_URL in prisma.schema. Add one with --database.\n",
		"It reads these variables: DATABASE_URL, STRIPE_KEY\n",
		"Worth knowing: The Stripe key is read at build time.\n",
		"Setting up web. Tael is reading the repository and will open a setup pull request.\n",
		"Adding a Tael Managed Postgres for it; its connection arrives as DATABASE_URL.\n",
		"  ✓ Read the repository.\n",
		"  ✓ Wrote the setup.\n",
		"The setup pull request is ready: https://github.com/acme/web/pull/3\n",
		"Merged it; web is going live. Follow it with `tael logs web -f`.\n",
	)
	mustSpeakTael(t, output)

	analyse := decodeBody(t, lastRequest(recorded, http.MethodPost, "/api/v1/repos/analyse"))
	if analyse["repo_full_name"] != "acme/web" || analyse["default_branch"] != "main" || analyse["installation_id"] != float64(42) {
		t.Fatalf("analyse body = %v", analyse)
	}
	create := decodeBody(t, lastRequest(recorded, http.MethodPost, "/api/v1/apps"))
	if create["repo_full_name"] != "acme/web" || create["default_branch"] != "main" || create["installation_id"] != float64(42) || create["name"] != nil {
		t.Fatalf("create body = %v, want the picked repository, its branch and installation, and no name", create)
	}
	database := decodeBody(t, lastRequest(recorded, http.MethodPost, "/api/v1/solutions"))
	if database["solution_key"] != "postgres" || database["preset"] != "small" || database["app_id"] != "app_9" {
		t.Fatalf("database body = %v", database)
	}
	if lastRequest(recorded, http.MethodPost, "/api/v1/apps/app_9/go-live") == nil {
		t.Fatalf("--go-live did not merge the pull request")
	}
}

func TestNewCarriesOnWhenTheReadingIsOff(t *testing.T) {
	server, recorded := newAPIServer(t,
		route{http.MethodGet, "/api/v1/github/repos", http.StatusOK, reposJSON},
		route{http.MethodPost, "/api/v1/repos/analyse", http.StatusNotImplemented, `{"detail":"Repository analysis is not configured on this deployment."}`},
		route{http.MethodPost, "/api/v1/apps", http.StatusCreated, `{"id":"app_9"}`},
	)
	output, runError := runCommand(t, server, "new", "--repo", "acme/api", "--branch", "release", "--name", "backend", "--no-follow")
	if runError != nil {
		t.Fatalf("tael new --no-follow: %v", runError)
	}
	mustContain(t, output, "Tael could not read it first; carrying on.\n", "Setting up backend.", "Follow it with `tael setup backend`.\n")
	create := decodeBody(t, lastRequest(recorded, http.MethodPost, "/api/v1/apps"))
	if create["name"] != "backend" || create["default_branch"] != "release" {
		t.Fatalf("create body = %v, want the name and branch given", create)
	}
}

func TestNewRefusals(t *testing.T) {
	server, recorded := newAPIServer(t,
		route{http.MethodGet, "/api/v1/github/repos", http.StatusOK, reposJSON},
		route{http.MethodPost, "/api/v1/repos/analyse", http.StatusNotImplemented, `{"detail":"off"}`},
		route{http.MethodPost, "/api/v1/apps", http.StatusConflict, `{"detail":"acme/web is already set up as an app in this workspace."}`},
	)
	_, unknown := runCommand(t, server, "new", "--repo", "acme/nope")
	if unknown == nil || exitCodeFor(unknown) != exitUsage || !strings.Contains(unknown.Error(), "it can see: acme/web, acme/api") {
		t.Fatalf("tael new on an unseen repo = %v, want a usage error listing what Tael can see", unknown)
	}
	if lastRequest(recorded, http.MethodPost, "/api/v1/apps") != nil {
		t.Fatalf("an unseen repository must not create an app")
	}
	_, malformed := runCommand(t, server, "new", "--repo", "web")
	if malformed == nil || exitCodeFor(malformed) != exitUsage {
		t.Fatalf("tael new --repo web = %v, want a usage error", malformed)
	}
	_, missing := runCommand(t, server, "new")
	if missing == nil || exitCodeFor(missing) != exitUsage {
		t.Fatalf("tael new without --repo = %v, want a usage error", missing)
	}
	_, conflict := runCommand(t, server, "new", "--repo", "acme/web")
	if conflict == nil || !strings.Contains(conflict.Error(), "already set up as an app") {
		t.Fatalf("tael new on an onboarded repo = %v, want the 409 sentence", conflict)
	}
}

func TestNarrateSetupEvent(t *testing.T) {
	line, refresh := narrateSetupEvent(event("app_analysis_progress", map[string]any{"app_id": "app_9", "status": "analyzing"}), "app_9")
	if line != "" || !refresh {
		t.Fatalf("progress for this app = (%q, %v), want a refresh", line, refresh)
	}
	if _, refresh := narrateSetupEvent(event("app_analysis_progress", map[string]any{"app_id": "other"}), "app_9"); refresh {
		t.Fatalf("progress for another app must not refresh")
	}
	line, _ = narrateSetupEvent(event("step_started", map[string]any{"name": "app_watch_generation"}), "app_9")
	if line != "Reading your repo and working out what it needs." {
		t.Fatalf("step narration = %q", line)
	}
	if line, _ := narrateSetupEvent(event("step_started", map[string]any{"name": "workspace_create_runtime"}), "app_9"); line != "" {
		t.Fatalf("an unrelated step narrated as %q", line)
	}
	for name, narration := range setupStepNarration {
		if strings.Contains(narration, "_") || strings.Contains(strings.ToLower(narration), "ankra") {
			t.Errorf("setupStepNarration[%s] = %q leaks an identifier", name, narration)
		}
	}
	rendered := renderAnalysis(&client.RepoAnalysis{Framework: "", NeedsDatabase: false, DatabaseReason: "nothing reads a database"})
	if !strings.HasPrefix(rendered, "This looks like a web app.\n") || !strings.Contains(rendered, "It does not need a database: nothing reads a database.\n") {
		t.Fatalf("renderAnalysis = %q", rendered)
	}
}
