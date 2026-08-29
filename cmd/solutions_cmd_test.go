package cmd

import (
	"net/http"
	"strings"
	"testing"
)

const installedSolutionsJSON = `{"solutions":[{"id":"sol_1","solution_key":"postgres","name":"Tael Managed Postgres for web","instance":"tael-postgres-web","status":"ready","update_available":true,"app":{"id":"app_1","name":"web"},"bindings":[],"connection":[{"name":"DATABASE_URL","value":"postgres://tael:••••@db:5432/web","secret":true},{"name":"PGHOST","value":"db","secret":false}]}]}`

func TestSolutionsCatalog(t *testing.T) {
	server, _ := newAPIServer(t,
		route{http.MethodGet, "/api/v1/solutions/catalog", http.StatusOK, `{"promise":"Tael installs it, watches it and keeps it up to date.","plan":"launch","solutions":[{"key":"postgres","name":"Tael Managed Postgres","category":"database","availability":{"state":"available","label":"Add"},"default_preset":"small","presets":[{"key":"small","label":"Small"},{"key":"medium","label":"Medium"}]},{"key":"monitoring","name":"Tael Managed Monitoring","category":"observability","included":false,"availability":{"state":"plan_required","label":"Available on Scale"},"default_preset":"small","presets":[{"key":"small","label":"Small"}]}]}`},
	)
	output, runError := runCommand(t, server, "solutions", "catalog")
	if runError != nil {
		t.Fatalf("tael solutions catalog: %v", runError)
	}
	mustContain(t, output,
		"Tael installs it, watches it and keeps it up to date.\n",
		"KEY         NAME                     CATEGORY       AVAILABILITY        SIZES\n",
		"postgres    Tael Managed Postgres    database       can be added        small (default), medium\n",
		"monitoring  Tael Managed Monitoring  observability  Available on Scale  small (default)\n",
		"Add one with `tael solutions add <key>`. Plan: launch.\n",
	)
	mustSpeakTael(t, output)
}

func TestSolutionsUpgradeRetryAndConnection(t *testing.T) {
	server, recorded := newAPIServer(t,
		route{http.MethodGet, "/api/v1/solutions", http.StatusOK, installedSolutionsJSON},
		route{http.MethodPost, "/api/v1/solutions/sol_1/upgrade", http.StatusAccepted, `{"id":"sol_1","operation_id":"op_1","status":"updating"}`},
		route{http.MethodPost, "/api/v1/solutions/sol_1/retry", http.StatusConflict, `{"detail":"Tael Managed Postgres for web did not fail; there is nothing to retry."}`},
		route{http.MethodGet, "/api/v1/solutions/sol_1/connection", http.StatusOK, `{"variables":[{"name":"DATABASE_URL","value":"postgres://tael:••••@db:5432/web","secret":true},{"name":"PGHOST","value":"db","secret":false}],"revealed":false}`},
	)
	output, upgradeError := runCommand(t, server, "solutions", "upgrade", "tael-postgres-web")
	if upgradeError != nil {
		t.Fatalf("tael solutions upgrade: %v", upgradeError)
	}
	mustContain(t, output, "Updating Tael Managed Postgres for web to the newer version. Follow it with `tael solutions status sol_1`.\n")
	if lastRequest(recorded, http.MethodPost, "/api/v1/solutions/sol_1/upgrade") == nil {
		t.Fatalf("upgrade did not POST")
	}

	_, retryError := runCommand(t, server, "solutions", "retry", "Tael Managed Postgres for web")
	if retryError == nil || !strings.Contains(retryError.Error(), "nothing to retry") {
		t.Fatalf("tael solutions retry on a healthy solution = %v, want the 409 sentence", retryError)
	}

	output, connectionError := runCommand(t, server, "solutions", "connection", "sol_1")
	if connectionError != nil {
		t.Fatalf("tael solutions connection: %v", connectionError)
	}
	mustContain(t, output,
		"Connection for Tael Managed Postgres for web (secrets masked):\n",
		"  DATABASE_URL  postgres://tael:••••@db:5432/web  secret\n",
		"  PGHOST        db\n",
		"Revealing the values is a browser action",
	)
	if request := lastRequest(recorded, http.MethodGet, "/api/v1/solutions/sol_1/connection"); request == nil || strings.Contains(request.Path, "reveal") {
		t.Fatalf("connection must never ask to reveal: %+v", request)
	}
	mustSpeakTael(t, output)

	_, unknown := runCommand(t, server, "solutions", "connection", "nothing")
	if unknown == nil || exitCodeFor(unknown) != exitUsage {
		t.Fatalf("tael solutions connection nothing = %v, want a usage error", unknown)
	}
}
