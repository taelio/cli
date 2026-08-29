package cmd

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"tael.io/cli/internal/client"
)

const architectureGraphJSON = `{
  "nodes": [
    {"id":"runtime","kind":"runtime","title":"Dedicated machine","subtitle":"Hetzner · ready","status":"ready","tone":"live","group":null,"href":"/settings/runtime","facts":[{"label":"Plan","value":"Launch"}],"actions":[{"kind":"link","label":"Runtime settings","href":"/settings/runtime"}]},
    {"id":"app:app_1","kind":"app","title":"website-demo","subtitle":"taelio/website-demo · Next.js","status":"live","tone":"live","group":null,"href":"/app/app_1","facts":[{"label":"Branch","value":"main"}],"actions":[{"kind":"link","label":"Open app","href":"/app/app_1"},{"kind":"deploy","label":"Deploy","app_id":"app_1"}]},
    {"id":"domain:website-demo.tael.site","kind":"domain","title":"website-demo.tael.site","subtitle":"Included address","status":"live","tone":"live","group":null,"href":"https://website-demo.tael.site","facts":[],"actions":[]},
    {"id":"app:app_2","kind":"app","title":"api","subtitle":"taelio/api · Go","status":"awaiting_review","tone":"warning","group":null,"href":"/app/app_2","facts":[],"actions":[]},
    {"id":"solution:sol_1","kind":"solution","title":"Tael Managed Postgres for website-demo","subtitle":"Small · for website-demo","status":"ready","tone":"live","group":null,"href":"/solutions/sol_1","facts":[{"label":"Kind","value":"database"}],"actions":[]},
    {"id":"solution:sol_2","kind":"solution","title":"Tael Managed Backups","subtitle":"backups","status":"installing","tone":"in_progress","group":null,"href":"/solutions/sol_2","facts":[],"actions":[]},
    {"id":"solution:sol_3","kind":"solution","title":"Tael Managed Object Storage","subtitle":"storage","status":"ready","tone":"live","group":null,"href":"/solutions/sol_3","facts":[],"actions":[]},
    {"id":"service:networking","kind":"service","title":"Networking","subtitle":"Ingress, certificates, DNS","status":"up","tone":"live","group":"runtime","facts":[],"actions":[]}
  ],
  "edges": [
    {"from":"app:app_1","to":"runtime","kind":"runs_on"},
    {"from":"domain:website-demo.tael.site","to":"app:app_1","kind":"routes"},
    {"from":"app:app_2","to":"runtime","kind":"runs_on"},
    {"from":"solution:sol_1","to":"runtime","kind":"runs_on"},
    {"from":"app:app_1","to":"solution:sol_1","kind":"reads","label":"DATABASE_URL +5"},
    {"from":"solution:sol_2","to":"runtime","kind":"runs_on"},
    {"from":"solution:sol_2","to":"solution:sol_3","kind":"requires"},
    {"from":"solution:sol_3","to":"runtime","kind":"runs_on"}
  ],
  "suggestions": [
    {"prompt":"Add Tael Managed Postgres for api","reason":"The code expects a database and none is connected."},
    {"prompt":"Add Tael Managed Monitoring","reason":"Nothing is watching metrics or logs yet."}
  ],
  "generated_at": "2026-08-29T10:00:00Z"
}`

func TestArchitecture(t *testing.T) {
	server, _ := newAPIServer(t, route{http.MethodGet, "/api/v1/architecture", http.StatusOK, architectureGraphJSON})
	output, runError := runCommand(t, server, "architecture")
	if runError != nil {
		t.Fatalf("tael architecture: %v", runError)
	}
	want := strings.Join([]string{
		"Addresses",
		"  ● live  website-demo.tael.site — Included address",
		"      routes to website-demo",
		"Apps",
		"  ● live  website-demo — taelio/website-demo · Next.js",
		"      runs on Dedicated machine",
		"      reads DATABASE_URL +5 from Tael Managed Postgres for website-demo",
		"  ▲ awaiting review  api — taelio/api · Go",
		"      runs on Dedicated machine",
		"Solutions",
		"  ● ready  Tael Managed Postgres for website-demo — Small · for website-demo",
		"      runs on Dedicated machine",
		"  ◐ installing  Tael Managed Backups — backups",
		"      runs on Dedicated machine",
		"      requires Tael Managed Object Storage",
		"  ● ready  Tael Managed Object Storage — storage",
		"      runs on Dedicated machine",
		"Runtime",
		"  ● ready  Dedicated machine — Hetzner · ready",
		"    ● up  Networking — Ingress, certificates, DNS",
		"Suggestions",
		"  - Add Tael Managed Postgres for api — The code expects a database and none is connected.",
		"  - Add Tael Managed Monitoring — Nothing is watching metrics or logs yet.",
		"Ask for one with `tael plan \"<suggestion>\"`.",
		"",
	}, "\n")
	if output != want {
		t.Errorf("tael architecture output:\n%s\nwant:\n%s", output, want)
	}
	mustSpeakTael(t, output)

	// --app keeps the app and what its edges reach, nothing else.
	output, appError := runCommand(t, server, "architecture", "--app", "website-demo")
	if appError != nil {
		t.Fatalf("tael architecture --app: %v", appError)
	}
	mustContain(t, output,
		"  ● live  website-demo.tael.site — Included address\n",
		"  ● live  website-demo — taelio/website-demo · Next.js\n",
		"  ● ready  Tael Managed Postgres for website-demo — Small · for website-demo\n",
		"  ● ready  Dedicated machine — Hetzner · ready\n",
	)
	for _, absent := range []string{"api —", "Backups", "Object Storage", "Networking", "Monitoring"} {
		if strings.Contains(output, absent) {
			t.Errorf("--app website-demo still shows %q:\n%s", absent, output)
		}
	}
	mustSpeakTael(t, output)

	_, unknown := runCommand(t, server, "architecture", "--app", "nothing")
	if unknown == nil || exitCodeFor(unknown) != exitUsage || !strings.Contains(unknown.Error(), "apps: website-demo, api") {
		t.Fatalf("tael architecture --app nothing = %v, want a usage error listing the apps", unknown)
	}

	output, jsonError := runCommand(t, server, "architecture", "-o", "json")
	if jsonError != nil {
		t.Fatalf("tael architecture -o json: %v", jsonError)
	}
	var graph client.ArchitectureGraph
	if unmarshalError := json.Unmarshal([]byte(output), &graph); unmarshalError != nil || len(graph.Nodes) != 8 || len(graph.Edges) != 8 || graph.GeneratedAt != "2026-08-29T10:00:00Z" {
		t.Fatalf("tael architecture -o json = %q, %v", output, unmarshalError)
	}
}

func TestArchitectureRuntimeDidNotAnswerAndRefusal(t *testing.T) {
	quiet, _ := newAPIServer(t, route{http.MethodGet, "/api/v1/architecture", http.StatusOK,
		`{"nodes":[{"id":"runtime","kind":"runtime","title":"Runtime","subtitle":"Being set up","status":"provisioning","tone":"in_progress","group":null,"facts":[],"actions":[]}],"edges":[],"suggestions":[{"prompt":"Connect a repo","reason":"Nothing is running yet."}],"runtime_services_unavailable":true,"generated_at":"2026-08-29T10:00:00Z"}`})
	output, runError := runCommand(t, quiet, "architecture")
	if runError != nil {
		t.Fatalf("tael architecture: %v", runError)
	}
	mustContain(t, output,
		"Runtime\n  ◐ provisioning  Runtime — Being set up\n    The runtime did not answer; services will appear when it does.\n",
		"Suggestions\n  - Connect a repo — Nothing is running yet.\n",
	)
	mustSpeakTael(t, output)

	refusing, _ := newAPIServer(t, route{http.MethodGet, "/api/v1/architecture", http.StatusServiceUnavailable, `{"detail":"Tael cannot read the workspace just now; try again in a minute."}`})
	_, refusal := runCommand(t, refusing, "architecture")
	if refusal == nil || !strings.Contains(refusal.Error(), "cannot read the workspace") || exitCodeFor(refusal) != exitError {
		t.Fatalf("tael architecture on 503 = %v, want the sentence and exit 1", refusal)
	}
}

const architecturePlanJSON = `{
  "summary": "Tael would add a database for website-demo and hand it the connection details.",
  "changes": [
    {"kind":"add_solution","solution_key":"postgres","preset":"small","app_id":"app_1","title":"Add Tael Managed Postgres for website-demo","detail":"A database made for website-demo; it reads DATABASE_URL on its next deploy."},
    {"kind":"add_solution","solution_key":"monitoring","preset":"small","title":"Add Tael Managed Monitoring","detail":"Metrics and logs for every app.","blocked":"Available on Launch"},
    {"kind":"new_app","repo":"taelio/api","branch":"main","title":"Connect taelio/api","detail":"Tael reads the repository, opens a setup pull request and deploys api once you approve it."}
  ],
  "questions": ["Should the database be small or medium?"],
  "preview": {"nodes":[],"edges":[]},
  "model": "planner"
}`

func TestPlanWithASentence(t *testing.T) {
	server, recorded := newAPIServer(t, route{http.MethodPost, "/api/v1/architecture/plan", http.StatusOK, architecturePlanJSON})
	configPath := filepath.Join(t.TempDir(), "tael.yaml")
	output, planError := runCommandKeepingConfig(t, server, configPath, "plan", "Add a database for website-demo")
	if planError != nil {
		t.Fatalf("tael plan: %v", planError)
	}
	want := strings.Join([]string{
		"Tael would add a database for website-demo and hand it the connection details.",
		"",
		"  1. Add Tael Managed Postgres for website-demo",
		"     A database made for website-demo; it reads DATABASE_URL on its next deploy.",
		"  2. Add Tael Managed Monitoring",
		"     Metrics and logs for every app.",
		"     Blocked: Available on Launch",
		"  3. Connect taelio/api",
		"     Tael reads the repository, opens a setup pull request and deploys api once you approve it.",
		"",
		"Questions:",
		"  - Should the database be small or medium?",
		"",
		"Nothing runs until you say `tael build`.",
		"",
	}, "\n")
	if output != want {
		t.Errorf("tael plan output:\n%s\nwant:\n%s", output, want)
	}
	mustSpeakTael(t, output)
	if body := decodeBody(t, lastRequest(recorded, http.MethodPost, "/api/v1/architecture/plan")); body["prompt"] != "Add a database for website-demo" {
		t.Fatalf("plan body = %v", body)
	}

	// The plan is kept beside the config file for `tael build`.
	kept, readError := os.ReadFile(filepath.Join(filepath.Dir(configPath), lastPlanFileName))
	if readError != nil {
		t.Fatalf("last plan not kept: %v", readError)
	}
	var plan client.ArchitecturePlan
	if unmarshalError := json.Unmarshal(kept, &plan); unmarshalError != nil || len(plan.Changes) != 3 || plan.Changes[1].Blocked != "Available on Launch" {
		t.Fatalf("kept plan = %s, %v", kept, unmarshalError)
	}

	output, jsonError := runCommand(t, server, "plan", "Add a database", "--json")
	if jsonError != nil || !strings.Contains(output, `"summary": "Tael would add a database`) {
		t.Fatalf("tael plan --json = %q, %v", output, jsonError)
	}

	_, empty := runCommand(t, server, "plan", "   ")
	if empty == nil || exitCodeFor(empty) != exitUsage {
		t.Fatalf("tael plan with nothing to plan = %v, want a usage error", empty)
	}
}

func TestPlanWhenTaelCannotPlan(t *testing.T) {
	noModel, _ := newAPIServer(t, route{http.MethodPost, "/api/v1/architecture/plan", http.StatusNotImplemented, `{"detail":"Tael cannot plan changes on this deployment yet."}`})
	_, planError := runCommand(t, noModel, "plan", "Add a database")
	if !errors.Is(planError, client.ErrPlanningUnavailable) || planError.Error() != "Tael cannot plan changes on this deployment yet" || exitCodeFor(planError) != exitError {
		t.Fatalf("tael plan on 501 = %v, want the sentence and exit 1", planError)
	}

	modelDown, _ := newAPIServer(t, route{http.MethodPost, "/api/v1/architecture/plan", http.StatusBadGateway, `{"detail":"Tael could not write a plan just now; try again in a minute."}`})
	_, downError := runCommand(t, modelDown, "plan", "Add a database")
	if downError == nil || !strings.Contains(downError.Error(), "could not write a plan just now") || exitCodeFor(downError) != exitError {
		t.Fatalf("tael plan on 502 = %v, want the sentence and exit 1", downError)
	}
}

// keepPlan writes the plan a test builds from, the way `tael plan` would.
func keepPlan(t *testing.T, configPath string, planJSON string) string {
	t.Helper()
	path := filepath.Join(filepath.Dir(configPath), lastPlanFileName)
	if writeError := os.WriteFile(path, []byte(planJSON), 0o600); writeError != nil {
		t.Fatalf("keep plan: %v", writeError)
	}
	return path
}

func setTerminal(t *testing.T, terminal bool) {
	t.Helper()
	previous := stdinIsTerminal
	stdinIsTerminal = func() bool { return terminal }
	t.Cleanup(func() { stdinIsTerminal = previous })
}

func TestBuild(t *testing.T) {
	setTerminal(t, false)
	configPath := filepath.Join(t.TempDir(), "tael.yaml")

	// Everything applied: one line per change, the plan file is done with.
	applied, recorded := newAPIServer(t, route{http.MethodPost, "/api/v1/architecture/apply", http.StatusOK,
		`{"applied":[{"change":{"kind":"add_solution","solution_key":"postgres","preset":"small","app_id":"app_1","title":"Add Tael Managed Postgres for website-demo","detail":"A database made for website-demo."},"id":"sol_9","operation_id":"op_9","task_id":"task_9"},{"change":{"kind":"connect","solution_key":"object-storage","app_id":"app_1","title":"Connect website-demo to Tael Managed Object Storage","detail":"Hand website-demo the connection details."},"id":"sol_3","operation_id":"op_10"}],"refused":[]}`})
	planPath := keepPlan(t, configPath, `{"summary":"Tael would add a database and connect the storage.","changes":[
		{"kind":"add_solution","solution_key":"postgres","preset":"small","app_id":"app_1","title":"Add Tael Managed Postgres for website-demo","detail":"A database made for website-demo."},
		{"kind":"add_solution","solution_key":"monitoring","preset":"small","title":"Add Tael Managed Monitoring","detail":"Metrics and logs.","blocked":"Available on Launch"},
		{"kind":"connect","solution_key":"object-storage","app_id":"app_1","title":"Connect website-demo to Tael Managed Object Storage","detail":"Hand website-demo the connection details."}
	],"questions":[],"preview":{"nodes":[],"edges":[]},"model":"planner"}`)
	output, buildError := runCommandKeepingConfig(t, applied, configPath, "build", "--yes")
	if buildError != nil {
		t.Fatalf("tael build --yes: %v", buildError)
	}
	want := strings.Join([]string{
		"Tael would add a database and connect the storage.",
		"  1. Add Tael Managed Postgres for website-demo",
		"     A database made for website-demo.",
		"  2. Add Tael Managed Monitoring",
		"     Metrics and logs.",
		"     Blocked: Available on Launch",
		"  3. Connect website-demo to Tael Managed Object Storage",
		"     Hand website-demo the connection details.",
		"Skipping 1 change (blocked).",
		"Adding Tael Managed Postgres for website-demo — Tael will tell you when it is ready.",
		"Connecting website-demo to Tael Managed Object Storage — Tael will tell you when it is ready.",
		"Follow them with `tael tasks`.",
		"",
	}, "\n")
	if output != want {
		t.Errorf("tael build output:\n%s\nwant:\n%s", output, want)
	}
	mustSpeakTael(t, output)
	body := decodeBody(t, lastRequest(recorded, http.MethodPost, "/api/v1/architecture/apply"))
	changes, _ := body["changes"].([]any)
	if len(changes) != 2 {
		t.Fatalf("apply body sent %d changes, want the 2 that are not blocked: %v", len(changes), body)
	}
	first, _ := changes[0].(map[string]any)
	if first["kind"] != "add_solution" || first["solution_key"] != "postgres" || first["preset"] != "small" || first["app_id"] != "app_1" || first["blocked"] != nil {
		t.Fatalf("first change sent = %v", first)
	}
	if _, statError := os.Stat(planPath); !errors.Is(statError, os.ErrNotExist) {
		t.Fatalf("the built plan is still at %s (%v); a second build would do it twice", planPath, statError)
	}

	// Nothing kept: say what to do first.
	_, nothing := runCommandKeepingConfig(t, applied, configPath, "build", "--yes")
	if nothing == nil || exitCodeFor(nothing) != exitUsage || !strings.Contains(nothing.Error(), "no plan yet") {
		t.Fatalf("tael build with no plan = %v, want a usage error", nothing)
	}
}

func TestBuildRefusalAndPlanFile(t *testing.T) {
	setTerminal(t, false)
	configPath := filepath.Join(t.TempDir(), "tael.yaml")
	refusing, _ := newAPIServer(t, route{http.MethodPost, "/api/v1/architecture/apply", http.StatusOK,
		`{"applied":[{"change":{"kind":"add_solution","solution_key":"postgres","app_id":"app_1","title":"Add Tael Managed Postgres for website-demo","detail":"A database."},"id":"sol_9","operation_id":"op_9"}],"refused":[{"change":{"kind":"new_app","repo":"taelio/api","branch":"main","title":"Connect taelio/api","detail":"Tael reads the repository."},"reason":"Connect the repo from New app; Tael reads it first."}]}`})
	planFile := filepath.Join(t.TempDir(), "my-plan.json")
	if writeError := os.WriteFile(planFile, []byte(`{"changes":[
		{"kind":"add_solution","solution_key":"postgres","app_id":"app_1","title":"Add Tael Managed Postgres for website-demo","detail":"A database."},
		{"kind":"new_app","repo":"taelio/api","branch":"main","title":"Connect taelio/api","detail":"Tael reads the repository."},
		{"kind":"add_solution","solution_key":"backups","title":"Add Tael Managed Backups","detail":"Backups on a schedule."}
	]}`), 0o600); writeError != nil {
		t.Fatalf("write plan file: %v", writeError)
	}
	output, buildError := runCommandKeepingConfig(t, refusing, configPath, "build", "--plan", planFile, "--yes")
	if buildError == nil || exitCodeFor(buildError) != exitError || !strings.Contains(buildError.Error(), "1 change refused") {
		t.Fatalf("tael build with a refusal = %v, want exit 1", buildError)
	}
	mustContain(t, output,
		"Adding Tael Managed Postgres for website-demo — Tael will tell you when it is ready.\n",
		"Refused: Connect taelio/api — Connect the repo from New app; Tael reads it first.\n",
		"1 change not attempted; Tael stops at the first refusal.\n",
	)
	mustSpeakTael(t, output)
	if _, statError := os.Stat(planFile); statError != nil {
		t.Fatalf("a plan given with --plan must be left alone: %v", statError)
	}

	// -o json prints the outcome and still exits 1.
	output, jsonError := runCommandKeepingConfig(t, refusing, configPath, "build", "--plan", planFile, "--yes", "-o", "json")
	if jsonError == nil || exitCodeFor(jsonError) != exitError {
		t.Fatalf("tael build -o json with a refusal = %v, want exit 1", jsonError)
	}
	var outcome client.ArchitectureOutcome
	if unmarshalError := json.Unmarshal([]byte(output), &outcome); unmarshalError != nil || len(outcome.Applied) != 1 || len(outcome.Refused) != 1 {
		t.Fatalf("tael build -o json = %q, %v", output, unmarshalError)
	}

	_, missing := runCommandKeepingConfig(t, refusing, configPath, "build", "--plan", filepath.Join(t.TempDir(), "none.json"), "--yes")
	if missing == nil || exitCodeFor(missing) != exitUsage {
		t.Fatalf("tael build --plan with a missing file = %v, want a usage error", missing)
	}
}

func TestBuildAsksBeforeItStarts(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "tael.yaml")
	server, recorded := newAPIServer(t, route{http.MethodPost, "/api/v1/architecture/apply", http.StatusOK,
		`{"applied":[{"change":{"kind":"add_solution","solution_key":"postgres","app_id":"app_1","title":"Add Tael Managed Postgres for website-demo","detail":"A database."},"id":"sol_9","operation_id":"op_9"}],"refused":[]}`})
	planJSON := `{"changes":[{"kind":"add_solution","solution_key":"postgres","app_id":"app_1","title":"Add Tael Managed Postgres for website-demo","detail":"A database."}]}`

	// Not a terminal and no --yes: refuse rather than guess, and send nothing.
	setTerminal(t, false)
	keepPlan(t, configPath, planJSON)
	_, refusal := runCommandKeepingConfig(t, server, configPath, "build")
	if refusal == nil || exitCodeFor(refusal) != exitUsage || !strings.Contains(refusal.Error(), "run again with --yes") {
		t.Fatalf("tael build without a terminal = %v, want exit 2 and the --yes hint", refusal)
	}
	if lastRequest(recorded, http.MethodPost, "/api/v1/architecture/apply") != nil {
		t.Fatalf("a refused build must not apply anything")
	}

	// A terminal: ask, and a no builds nothing.
	setTerminal(t, true)
	rootCmd.SetIn(strings.NewReader("n\n"))
	t.Cleanup(func() { rootCmd.SetIn(nil) })
	output, declined := runCommandKeepingConfig(t, server, configPath, "build")
	if declined != nil || !strings.Contains(output, "Build 1 change? [y/N] ") || !strings.HasSuffix(output, "Nothing built.\n") {
		t.Fatalf("tael build answered no = %q, %v", output, declined)
	}
	if lastRequest(recorded, http.MethodPost, "/api/v1/architecture/apply") != nil {
		t.Fatalf("a declined build must not apply anything")
	}

	// A yes builds.
	rootCmd.SetIn(strings.NewReader("y\n"))
	output, accepted := runCommandKeepingConfig(t, server, configPath, "build")
	if accepted != nil {
		t.Fatalf("tael build answered yes: %v", accepted)
	}
	mustContain(t, output, "Build 1 change? [y/N] Adding Tael Managed Postgres for website-demo — Tael will tell you when it is ready.\n")
	if lastRequest(recorded, http.MethodPost, "/api/v1/architecture/apply") == nil {
		t.Fatalf("an accepted build must apply")
	}
	mustSpeakTael(t, output)
}

func TestPlanBuild(t *testing.T) {
	setTerminal(t, false)
	configPath := filepath.Join(t.TempDir(), "tael.yaml")
	server, recorded := newAPIServer(t,
		route{http.MethodPost, "/api/v1/architecture/plan", http.StatusOK, architecturePlanJSON},
		route{http.MethodPost, "/api/v1/architecture/apply", http.StatusOK,
			`{"applied":[{"change":{"kind":"add_solution","solution_key":"postgres","preset":"small","app_id":"app_1","title":"Add Tael Managed Postgres for website-demo","detail":"A database made for website-demo."},"id":"sol_9","operation_id":"op_9"},{"change":{"kind":"new_app","repo":"taelio/api","branch":"main","title":"Connect taelio/api","detail":"Tael reads the repository."},"id":"app_3","operation_id":"op_11"}],"refused":[]}`},
	)
	output, planError := runCommandKeepingConfig(t, server, configPath, "plan", "Add a database for website-demo", "--build", "--yes")
	if planError != nil {
		t.Fatalf("tael plan --build --yes: %v", planError)
	}
	mustContain(t, output,
		"Tael would add a database for website-demo and hand it the connection details.\n",
		"  1. Add Tael Managed Postgres for website-demo\n",
		"Skipping 1 change (blocked).\n",
		"Adding Tael Managed Postgres for website-demo — Tael will tell you when it is ready.\n",
		"Connecting taelio/api — Tael will tell you when it is ready.\n",
	)
	if strings.Contains(output, "Nothing runs until") {
		t.Errorf("plan --build should not say nothing runs when the build follows:\n%s", output)
	}
	mustSpeakTael(t, output)
	body := decodeBody(t, lastRequest(recorded, http.MethodPost, "/api/v1/architecture/apply"))
	if changes, _ := body["changes"].([]any); len(changes) != 2 {
		t.Fatalf("plan --build sent %d changes, want the 2 that are not blocked", len(changes))
	}
	if _, statError := os.Stat(filepath.Join(filepath.Dir(configPath), lastPlanFileName)); !errors.Is(statError, os.ErrNotExist) {
		t.Fatalf("the built plan should be done with: %v", statError)
	}

	// Without a terminal and without --yes the plan is kept and nothing is built.
	_, refusal := runCommandKeepingConfig(t, server, configPath, "plan", "Add a database for website-demo", "--build")
	if refusal == nil || exitCodeFor(refusal) != exitUsage {
		t.Fatalf("tael plan --build without a terminal = %v, want exit 2", refusal)
	}
	if _, statError := os.Stat(filepath.Join(filepath.Dir(configPath), lastPlanFileName)); statError != nil {
		t.Fatalf("the plan should be kept for `tael build`: %v", statError)
	}

	// The workspace plan is still one word away.
	workspace, _ := newAPIServer(t,
		route{http.MethodGet, "/api/v1/status", http.StatusOK, `{"workspace_status":"ready","plan":"launch","apps_total":1,"apps_live":1,"open_incidents":0,"needs_you":0,"runtime_status":"ready","environment":null}`},
		route{http.MethodGet, "/api/v1/workspace/coupon", http.StatusOK, `{"grant":null}`},
	)
	output, workspaceError := runCommand(t, workspace, "plan")
	if workspaceError != nil || !strings.HasPrefix(output, "Plan:      launch\n") {
		t.Fatalf("tael plan alone = %q, %v", output, workspaceError)
	}
}
