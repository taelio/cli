package cmd

import (
	"net/http"
	"strings"
	"testing"
	"time"
)

const membersJSON = `{"members":[{"user_id":"u_1","name":"Dana","email":"dana@acme.io","github_login":"dana","avatar_url":null,"role":"owner","joined_at":"2026-08-01T10:00:00Z","invited_via":null},{"user_id":"u_2","name":null,"email":null,"github_login":"sam","avatar_url":null,"role":"member","joined_at":"2026-08-20T10:00:00Z","invited_via":"inv_1"}],"join_policy":"invite_only","your_role":"owner"}`

func TestMembersListsThePeople(t *testing.T) {
	previousLocal := time.Local
	time.Local = time.UTC
	t.Cleanup(func() { time.Local = previousLocal })

	server, _ := newAPIServer(t, route{http.MethodGet, "/api/v1/members", http.StatusOK, membersJSON})
	output, runError := runCommand(t, server, "members")
	if runError != nil {
		t.Fatalf("tael members: %v", runError)
	}
	mustContain(t, output,
		"NAME  GITHUB  EMAIL         ROLE    JOINED\n",
		"Dana  dana    dana@acme.io  owner   2026-08-01 10:00\n",
		"sam   sam     -             member  2026-08-20 10:00\n",
		"Joining: by invitation only. Invite someone with `tael invite`.\n",
	)
	mustSpeakTael(t, output)
}

func TestMembersRemoveResolvesThePersonAndHandsRefusalsThrough(t *testing.T) {
	server, recorded := newAPIServer(t,
		route{http.MethodGet, "/api/v1/members", http.StatusOK, membersJSON},
		route{http.MethodDelete, "/api/v1/members/u_2", http.StatusNoContent, ``},
		route{http.MethodDelete, "/api/v1/members/u_1", http.StatusConflict, `{"detail":"A workspace needs at least one owner."}`},
	)
	output, runError := runCommand(t, server, "members", "remove", "@Sam")
	if runError != nil {
		t.Fatalf("tael members remove: %v", runError)
	}
	mustContain(t, output, "Removed sam from the workspace.\n")
	if lastRequest(recorded, http.MethodDelete, "/api/v1/members/u_2") == nil {
		t.Fatalf("remove did not DELETE the resolved member")
	}

	_, lastOwner := runCommand(t, server, "members", "remove", "dana@acme.io")
	if lastOwner == nil || !strings.Contains(lastOwner.Error(), "at least one owner") {
		t.Fatalf("removing the last owner = %v, want the API's sentence", lastOwner)
	}
	_, unknown := runCommand(t, server, "members", "remove", "nobody")
	if unknown == nil || exitCodeFor(unknown) != exitUsage || !strings.Contains(unknown.Error(), "members: Dana, sam") {
		t.Fatalf("removing nobody = %v, want a usage error listing the members", unknown)
	}
}

func TestInviteMakesALinkOnce(t *testing.T) {
	previousLocal := time.Local
	time.Local = time.UTC
	t.Cleanup(func() { time.Local = previousLocal })

	server, recorded := newAPIServer(t,
		route{http.MethodPost, "/api/v1/invitations", http.StatusCreated, `{"invitation":{"id":"inv_1","kind":"email","role":"admin","email":"kim@acme.io","github_login":null,"created_by":"u_1","expires_at":"2026-09-12T10:00:00Z","max_uses":1,"uses":0,"revoked_at":null,"created_at":"2026-08-29T10:00:00Z","status":"active"},"join_url":"https://tael.io/join/s3cret"}`},
	)
	output, runError := runCommand(t, server, "invite", "email", "kim@acme.io", "--role", "admin")
	if runError != nil {
		t.Fatalf("tael invite email: %v", runError)
	}
	mustContain(t, output,
		"Invited kim@acme.io as admin.\n",
		"Join link: https://tael.io/join/s3cret\n",
		"This is the only time the link is shown. It only works for them and expires 2026-09-12 10:00. Lose it and revoke it with `tael invites revoke inv_1`.\n",
	)
	body := decodeBody(t, lastRequest(recorded, http.MethodPost, "/api/v1/invitations"))
	if body["kind"] != "email" || body["email"] != "kim@acme.io" || body["role"] != "admin" || body["max_uses"] != nil {
		t.Fatalf("invite body = %v", body)
	}

	if _, linkError := runCommand(t, server, "invite", "link", "--max-uses", "3"); linkError != nil {
		t.Fatalf("tael invite link: %v", linkError)
	}
	body = decodeBody(t, lastRequest(recorded, http.MethodPost, "/api/v1/invitations"))
	if body["kind"] != "link" || body["max_uses"] != float64(3) || body["role"] != nil {
		t.Fatalf("link body = %v", body)
	}
	if _, githubError := runCommand(t, server, "invite", "github", "@kim"); githubError != nil {
		t.Fatalf("tael invite github: %v", githubError)
	}
	body = decodeBody(t, lastRequest(recorded, http.MethodPost, "/api/v1/invitations"))
	if body["kind"] != "github" || body["github_login"] != "kim" {
		t.Fatalf("github body = %v", body)
	}

	_, badRole := runCommand(t, server, "invite", "link", "--role", "boss")
	if badRole == nil || exitCodeFor(badRole) != exitUsage {
		t.Fatalf("tael invite --role boss = %v, want a usage error", badRole)
	}
}

func TestInviteRefusedForMembers(t *testing.T) {
	server, _ := newAPIServer(t,
		route{http.MethodPost, "/api/v1/invitations", http.StatusForbidden, `{"detail":"Only a workspace owner or admin can change who is in it."}`},
	)
	_, runError := runCommand(t, server, "invite", "link")
	if runError == nil || !strings.Contains(runError.Error(), "Only a workspace owner or admin") || exitCodeFor(runError) != exitAuth {
		t.Fatalf("tael invite as a member = %v (exit %d), want the 403 sentence", runError, exitCodeFor(runError))
	}
}

func TestInvitesListsAndRevokes(t *testing.T) {
	previousLocal := time.Local
	time.Local = time.UTC
	t.Cleanup(func() { time.Local = previousLocal })

	server, recorded := newAPIServer(t,
		route{http.MethodGet, "/api/v1/invitations", http.StatusOK, `{"invitations":[{"id":"inv_1","kind":"link","role":"member","email":null,"github_login":null,"created_by":"u_1","expires_at":"2026-09-12T10:00:00Z","max_uses":5,"uses":2,"revoked_at":null,"created_at":"2026-08-29T10:00:00Z","status":"active"},{"id":"inv_2","kind":"github","role":"admin","email":null,"github_login":"kim","created_by":"u_1","expires_at":"2026-09-12T10:00:00Z","max_uses":1,"uses":1,"revoked_at":null,"created_at":"2026-08-29T10:00:00Z","status":"used"}]}`},
		route{http.MethodDelete, "/api/v1/invitations/inv_1", http.StatusNoContent, ``},
		route{http.MethodDelete, "/api/v1/invitations/inv_9", http.StatusNotFound, `{"detail":"That invitation link is not valid."}`},
	)
	output, runError := runCommand(t, server, "invites")
	if runError != nil {
		t.Fatalf("tael invites: %v", runError)
	}
	mustContain(t, output, "ID     TO                    ROLE    STATUS  USED    EXPIRES\n", "inv_1  anyone with the link  member  active  2 of 5  2026-09-12 10:00\n", "inv_2  @kim                  admin   used    1 of 1")

	output, revokeError := runCommand(t, server, "invites", "revoke", "inv_1")
	if revokeError != nil || !strings.Contains(output, "Revoked.") || lastRequest(recorded, http.MethodDelete, "/api/v1/invitations/inv_1") == nil {
		t.Fatalf("tael invites revoke = %q, %v", output, revokeError)
	}
	_, missing := runCommand(t, server, "invites", "revoke", "inv_9")
	if missing == nil || !strings.Contains(missing.Error(), "not valid") {
		t.Fatalf("revoking an unknown invitation = %v", missing)
	}

	empty, _ := newAPIServer(t, route{http.MethodGet, "/api/v1/invitations", http.StatusOK, `{"invitations":[]}`})
	output, emptyError := runCommand(t, empty, "invites")
	if emptyError != nil || !strings.Contains(output, "No invitations.") {
		t.Fatalf("tael invites with none = %q, %v", output, emptyError)
	}
}

func TestTeamJoinPolicy(t *testing.T) {
	server, recorded := newAPIServer(t,
		route{http.MethodGet, "/api/v1/members", http.StatusOK, membersJSON},
		route{http.MethodPut, "/api/v1/workspace/join-policy", http.StatusOK, `{"join_policy":"github_repo_access"}`},
	)
	output, showError := runCommand(t, server, "team", "join-policy")
	if showError != nil {
		t.Fatalf("tael team join-policy: %v", showError)
	}
	mustContain(t, output, "Joining: by invitation only.\nChange it with --github-org on|off.\n")

	output, setError := runCommand(t, server, "team", "join-policy", "--github-org", "on")
	if setError != nil {
		t.Fatalf("tael team join-policy --github-org on: %v", setError)
	}
	mustContain(t, output, "Joining is now anyone with access to the workspace's GitHub repositories can join.\n")
	body := decodeBody(t, lastRequest(recorded, http.MethodPut, "/api/v1/workspace/join-policy"))
	if body["join_policy"] != "github_repo_access" {
		t.Fatalf("join-policy body = %v", body)
	}

	_, badValue := runCommand(t, server, "team", "join-policy", "--github-org", "maybe")
	if badValue == nil || exitCodeFor(badValue) != exitUsage {
		t.Fatalf("--github-org maybe = %v, want a usage error", badValue)
	}

	refused, _ := newAPIServer(t, route{http.MethodPut, "/api/v1/workspace/join-policy", http.StatusUnprocessableEntity, `{"detail":"Unknown join policy."}`})
	_, refusal := runCommand(t, refused, "team", "join-policy", "--github-org", "off")
	if refusal == nil || !strings.Contains(refusal.Error(), "Unknown join policy.") {
		t.Fatalf("refused join-policy = %v", refusal)
	}
}
