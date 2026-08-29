package cmd

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"tael.io/cli/internal/client"
)

const stacksTableJSON = `{"stacks":[
  {"id":"st_1","name":"checkout","apps":[
    {"id":"app_1","name":"web","status":"live","tone":"live"},
    {"id":"app_2","name":"api","status":"awaiting_review","tone":"warning"}
  ]},
  {"id":"st_2","name":"billing","apps":[]}
]}`

func TestStacksList(t *testing.T) {
	server, _ := newAPIServer(t, route{http.MethodGet, "/api/v1/stacks", http.StatusOK, stacksTableJSON})
	output, runError := runCommand(t, server, "stacks")
	if runError != nil {
		t.Fatalf("tael stacks: %v", runError)
	}
	want := strings.Join([]string{
		"NAME      APPS  MEMBERS",
		"checkout  2     web, api",
		"billing   0     -",
		"",
	}, "\n")
	if output != want {
		t.Errorf("tael stacks output:\n%s\nwant:\n%s", output, want)
	}
	mustSpeakTael(t, output)

	output, jsonError := runCommand(t, server, "stacks", "-o", "json")
	if jsonError != nil {
		t.Fatalf("tael stacks -o json: %v", jsonError)
	}
	var listResponse client.ListStacksResponse
	if unmarshalError := json.Unmarshal([]byte(output), &listResponse); unmarshalError != nil || len(listResponse.Stacks) != 2 || listResponse.Stacks[0].Apps[1].Name != "api" {
		t.Fatalf("tael stacks -o json = %q, %v", output, unmarshalError)
	}

	empty, _ := newAPIServer(t, route{http.MethodGet, "/api/v1/stacks", http.StatusOK, `{"stacks":[]}`})
	output, emptyError := runCommand(t, empty, "stacks")
	if emptyError != nil || output != "No stacks yet. Group apps with `tael stack new <name> --app <app>`.\n" {
		t.Fatalf("tael stacks with none = %q, %v", output, emptyError)
	}
	mustSpeakTael(t, output)
}

func TestStackNew(t *testing.T) {
	server, recorded := newAPIServer(t,
		route{http.MethodGet, "/api/v1/apps", http.StatusOK, appsForResolutionJSON},
		route{http.MethodPost, "/api/v1/stacks", http.StatusCreated,
			`{"id":"st_9","name":"checkout","apps":[{"id":"app_1","name":"website-demo","status":"live"},{"id":"app_2","name":"api","status":"awaiting_review"}]}`},
	)
	output, runError := runCommand(t, server, "stack", "new", "checkout", "--app", "website-demo", "--app", "api")
	if runError != nil {
		t.Fatalf("tael stack new: %v", runError)
	}
	if output != "Made the stack checkout with 2 apps. See it with `tael architecture --stack checkout`.\n" {
		t.Errorf("tael stack new output = %q", output)
	}
	mustSpeakTael(t, output)
	body := decodeBody(t, lastRequest(recorded, http.MethodPost, "/api/v1/stacks"))
	appIDs, _ := body["app_ids"].([]any)
	if body["name"] != "checkout" || len(appIDs) != 2 || appIDs[0] != "app_1" || appIDs[1] != "app_2" {
		t.Fatalf("create stack body = %v", body)
	}

	// Without apps the stack starts empty, and the body says nothing about
	// apps at all.
	output, bareError := runCommand(t, server, "stack", "new", "solo")
	if bareError != nil || output != "Made the stack solo. Put apps in it with `tael stack move <app> solo`.\n" {
		t.Fatalf("tael stack new solo = %q, %v", output, bareError)
	}
	mustSpeakTael(t, output)
	if body := decodeBody(t, lastRequest(recorded, http.MethodPost, "/api/v1/stacks")); body["app_ids"] != nil {
		t.Fatalf("create stack body without apps = %v", body)
	}

	_, unknown := runCommand(t, server, "stack", "new", "checkout", "--app", "nothing")
	if unknown == nil || exitCodeFor(unknown) != exitUsage || !strings.Contains(unknown.Error(), "apps: website-demo, api") {
		t.Fatalf("tael stack new with an unknown app = %v, want a usage error listing the apps", unknown)
	}

	output, jsonError := runCommand(t, server, "stack", "new", "checkout", "--app", "website-demo", "-o", "json")
	if jsonError != nil {
		t.Fatalf("tael stack new -o json: %v", jsonError)
	}
	var stack client.Stack
	if unmarshalError := json.Unmarshal([]byte(output), &stack); unmarshalError != nil || stack.ID != "st_9" || len(stack.Apps) != 2 {
		t.Fatalf("tael stack new -o json = %q, %v", output, unmarshalError)
	}

	// The API's refusal of a duplicate name comes through as its sentence.
	duplicate, _ := newAPIServer(t,
		route{http.MethodPost, "/api/v1/stacks", http.StatusConflict, `{"detail":"A stack called checkout already exists."}`},
	)
	_, conflict := runCommand(t, duplicate, "stack", "new", "checkout")
	if conflict == nil || exitCodeFor(conflict) != exitError || !strings.Contains(conflict.Error(), "already exists") {
		t.Fatalf("tael stack new on 409 = %v, want the API's sentence and exit 1", conflict)
	}
}

func TestStackMove(t *testing.T) {
	server, recorded := newAPIServer(t,
		route{http.MethodGet, "/api/v1/apps", http.StatusOK, appsForResolutionJSON},
		route{http.MethodGet, "/api/v1/stacks", http.StatusOK, stacksTableJSON},
		route{http.MethodPut, "/api/v1/apps/app_1/stack", http.StatusNoContent, ""},
	)
	output, runError := runCommand(t, server, "stack", "move", "website-demo", "checkout")
	if runError != nil {
		t.Fatalf("tael stack move: %v", runError)
	}
	if output != "Moved website-demo into checkout.\n" {
		t.Errorf("tael stack move output = %q", output)
	}
	mustSpeakTael(t, output)
	if body := decodeBody(t, lastRequest(recorded, http.MethodPut, "/api/v1/apps/app_1/stack")); body["stack_id"] != "st_1" {
		t.Fatalf("move body = %v", body)
	}

	// --none takes the app out: an explicit null.
	output, noneError := runCommand(t, server, "stack", "move", "website-demo", "--none")
	if noneError != nil || output != "Moved website-demo out of its stack; it stands on its own again.\n" {
		t.Fatalf("tael stack move --none = %q, %v", output, noneError)
	}
	mustSpeakTael(t, output)
	body := decodeBody(t, lastRequest(recorded, http.MethodPut, "/api/v1/apps/app_1/stack"))
	if value, present := body["stack_id"]; !present || value != nil {
		t.Fatalf("ungroup body = %v, want stack_id null", body)
	}

	_, missing := runCommand(t, server, "stack", "move", "website-demo")
	if missing == nil || exitCodeFor(missing) != exitUsage || !strings.Contains(missing.Error(), "say the stack") {
		t.Fatalf("tael stack move without a stack = %v, want a usage error", missing)
	}
	_, extra := runCommand(t, server, "stack", "move", "website-demo", "checkout", "--none")
	if extra == nil || exitCodeFor(extra) != exitUsage || !strings.Contains(extra.Error(), "--none takes only the app") {
		t.Fatalf("tael stack move with a stack and --none = %v, want a usage error", extra)
	}
	_, unknownStack := runCommand(t, server, "stack", "move", "website-demo", "nothing")
	if unknownStack == nil || exitCodeFor(unknownStack) != exitUsage || !strings.Contains(unknownStack.Error(), "stacks: checkout, billing") {
		t.Fatalf("tael stack move to an unknown stack = %v, want a usage error listing the stacks", unknownStack)
	}
	_, unknownApp := runCommand(t, server, "stack", "move", "nothing", "checkout")
	if unknownApp == nil || exitCodeFor(unknownApp) != exitUsage || !strings.Contains(unknownApp.Error(), "apps: website-demo, api") {
		t.Fatalf("tael stack move of an unknown app = %v, want a usage error listing the apps", unknownApp)
	}

	output, jsonError := runCommand(t, server, "stack", "move", "website-demo", "checkout", "-o", "json")
	if jsonError != nil || !strings.Contains(output, `"app_id": "app_1"`) || !strings.Contains(output, `"stack_id": "st_1"`) {
		t.Fatalf("tael stack move -o json = %q, %v", output, jsonError)
	}
}

func TestStackRename(t *testing.T) {
	server, recorded := newAPIServer(t,
		route{http.MethodGet, "/api/v1/stacks", http.StatusOK, stacksTableJSON},
		route{http.MethodPatch, "/api/v1/stacks/st_1", http.StatusOK, `{"id":"st_1","name":"payments","apps":[]}`},
	)
	output, runError := runCommand(t, server, "stack", "rename", "checkout", "payments")
	if runError != nil {
		t.Fatalf("tael stack rename: %v", runError)
	}
	if output != "Renamed checkout to payments.\n" {
		t.Errorf("tael stack rename output = %q", output)
	}
	mustSpeakTael(t, output)
	if body := decodeBody(t, lastRequest(recorded, http.MethodPatch, "/api/v1/stacks/st_1")); body["name"] != "payments" || len(body) != 1 {
		t.Fatalf("rename body = %v", body)
	}

	_, unknown := runCommand(t, server, "stack", "rename", "nothing", "payments")
	if unknown == nil || exitCodeFor(unknown) != exitUsage || !strings.Contains(unknown.Error(), "stacks: checkout, billing") {
		t.Fatalf("tael stack rename of an unknown stack = %v, want a usage error", unknown)
	}

	output, jsonError := runCommand(t, server, "stack", "rename", "checkout", "payments", "-o", "json")
	if jsonError != nil || !strings.Contains(output, `"name": "payments"`) {
		t.Fatalf("tael stack rename -o json = %q, %v", output, jsonError)
	}
}

func TestStackRemove(t *testing.T) {
	server, recorded := newAPIServer(t,
		route{http.MethodGet, "/api/v1/stacks", http.StatusOK, stacksTableJSON},
		route{http.MethodDelete, "/api/v1/stacks/st_1", http.StatusNoContent, ""},
	)

	// Not a terminal and no --yes: refuse rather than guess, and send
	// nothing.
	setTerminal(t, false)
	_, refusal := runCommand(t, server, "stack", "remove", "checkout")
	if refusal == nil || exitCodeFor(refusal) != exitUsage || !strings.Contains(refusal.Error(), "--yes") || !strings.Contains(refusal.Error(), "ungrouped") {
		t.Fatalf("tael stack remove without a terminal = %v, want exit 2 and the --yes hint", refusal)
	}
	if lastRequest(recorded, http.MethodDelete, "/api/v1/stacks/st_1") != nil {
		t.Fatalf("a refused remove must not delete anything")
	}

	// A terminal: the question says the apps stay, and a no removes nothing.
	setTerminal(t, true)
	rootCmd.SetIn(strings.NewReader("n\n"))
	t.Cleanup(func() { rootCmd.SetIn(nil) })
	output, declined := runCommand(t, server, "stack", "remove", "checkout")
	if declined != nil || !strings.Contains(output, "Remove the stack checkout? Its 2 apps stay, ungrouped. [y/N] ") || !strings.HasSuffix(output, "Nothing removed.\n") {
		t.Fatalf("tael stack remove answered no = %q, %v", output, declined)
	}
	if lastRequest(recorded, http.MethodDelete, "/api/v1/stacks/st_1") != nil {
		t.Fatalf("a declined remove must not delete anything")
	}

	// A yes removes.
	rootCmd.SetIn(strings.NewReader("y\n"))
	output, accepted := runCommand(t, server, "stack", "remove", "checkout")
	if accepted != nil || !strings.HasSuffix(output, "Removed the stack checkout. Its 2 apps stay, ungrouped.\n") {
		t.Fatalf("tael stack remove answered yes = %q, %v", output, accepted)
	}
	mustSpeakTael(t, output)
	if lastRequest(recorded, http.MethodDelete, "/api/v1/stacks/st_1") == nil {
		t.Fatalf("an accepted remove must delete")
	}

	// --yes skips the question, on a terminal or off it.
	setTerminal(t, false)
	output, forced := runCommand(t, server, "stack", "remove", "checkout", "--yes")
	if forced != nil || output != "Removed the stack checkout. Its 2 apps stay, ungrouped.\n" {
		t.Fatalf("tael stack remove --yes = %q, %v", output, forced)
	}
	mustSpeakTael(t, output)

	output, jsonError := runCommand(t, server, "stack", "remove", "checkout", "--yes", "-o", "json")
	if jsonError != nil || !strings.Contains(output, `"status": "removed"`) {
		t.Fatalf("tael stack remove -o json = %q, %v", output, jsonError)
	}
}

func TestLinkAndUnlink(t *testing.T) {
	server, recorded := newAPIServer(t,
		route{http.MethodGet, "/api/v1/apps", http.StatusOK, appsForResolutionJSON},
		route{http.MethodPost, "/api/v1/architecture/links", http.StatusCreated,
			`{"id":"lnk_1","from_app_id":"app_1","to_app_id":"app_2","label":"REST"}`},
		route{http.MethodDelete, "/api/v1/architecture/links", http.StatusNoContent, ""},
	)
	output, runError := runCommand(t, server, "link", "website-demo", "api", "--label", "REST")
	if runError != nil {
		t.Fatalf("tael link: %v", runError)
	}
	if output != "website-demo now calls api (REST). The picture shows it; take it back with `tael unlink website-demo api`.\n" {
		t.Errorf("tael link output = %q", output)
	}
	mustSpeakTael(t, output)
	if body := decodeBody(t, lastRequest(recorded, http.MethodPost, "/api/v1/architecture/links")); body["from_app_id"] != "app_1" || body["to_app_id"] != "app_2" || body["label"] != "REST" {
		t.Fatalf("link body = %v", body)
	}

	// Without a label the sentence and the body both stay plain.
	output, plainError := runCommand(t, server, "link", "website-demo", "api")
	if plainError != nil || !strings.HasPrefix(output, "website-demo now calls api. ") {
		t.Fatalf("tael link without a label = %q, %v", output, plainError)
	}
	if body := decodeBody(t, lastRequest(recorded, http.MethodPost, "/api/v1/architecture/links")); body["label"] != nil {
		t.Fatalf("link body without a label = %v", body)
	}

	_, self := runCommand(t, server, "link", "website-demo", "website-demo")
	if self == nil || exitCodeFor(self) != exitUsage || !strings.Contains(self.Error(), "does not call itself") {
		t.Fatalf("tael link to itself = %v, want a usage error", self)
	}
	if lastRequest(recorded, http.MethodPost, "/api/v1/architecture/links") == nil {
		t.Fatalf("expected the earlier link request to still be on record")
	}

	_, unknown := runCommand(t, server, "link", "website-demo", "nothing")
	if unknown == nil || exitCodeFor(unknown) != exitUsage || !strings.Contains(unknown.Error(), "apps: website-demo, api") {
		t.Fatalf("tael link to an unknown app = %v, want a usage error listing the apps", unknown)
	}

	output, jsonError := runCommand(t, server, "link", "website-demo", "api", "-o", "json")
	if jsonError != nil {
		t.Fatalf("tael link -o json: %v", jsonError)
	}
	var link client.ArchitectureLink
	if unmarshalError := json.Unmarshal([]byte(output), &link); unmarshalError != nil || link.ID != "lnk_1" || link.Label != "REST" {
		t.Fatalf("tael link -o json = %q, %v", output, unmarshalError)
	}

	output, unlinkError := runCommand(t, server, "unlink", "website-demo", "api")
	if unlinkError != nil || output != "website-demo no longer calls api.\n" {
		t.Fatalf("tael unlink = %q, %v", output, unlinkError)
	}
	mustSpeakTael(t, output)
	if body := decodeBody(t, lastRequest(recorded, http.MethodDelete, "/api/v1/architecture/links")); body["from_app_id"] != "app_1" || body["to_app_id"] != "app_2" {
		t.Fatalf("unlink body = %v", body)
	}

	// The API's refusal of a duplicate comes through as its sentence.
	duplicate, _ := newAPIServer(t,
		route{http.MethodGet, "/api/v1/apps", http.StatusOK, appsForResolutionJSON},
		route{http.MethodPost, "/api/v1/architecture/links", http.StatusConflict, `{"detail":"website-demo already calls api."}`},
	)
	_, conflict := runCommand(t, duplicate, "link", "website-demo", "api")
	if conflict == nil || exitCodeFor(conflict) != exitError || !strings.Contains(conflict.Error(), "already calls") {
		t.Fatalf("tael link on 409 = %v, want the API's sentence and exit 1", conflict)
	}
}
