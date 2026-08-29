package cmd

import (
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestPlanAndCoupon(t *testing.T) {
	previousLocal := time.Local
	time.Local = time.UTC
	t.Cleanup(func() { time.Local = previousLocal })

	server, recorded := newAPIServer(t,
		route{http.MethodGet, "/api/v1/status", http.StatusOK, `{"workspace_status":"ready","plan":"launch","apps_total":2,"apps_live":1,"open_incidents":0,"needs_you":1,"runtime_status":"ready","environment":{"status":"ready","tier":"free","kind":"playground","base_domain":"tael.site","pending_cluster":null,"upgrade_error":null}}`},
		route{http.MethodGet, "/api/v1/workspace/coupon", http.StatusOK, `{"grant":null}`},
		route{http.MethodPost, "/api/v1/workspace/coupon", http.StatusOK, `{"grant":{"code":"LAUNCH2026","plan":"launch","until":"2026-12-31T00:00:00Z","apps_included":3,"ai_tokens_included":1000000}}`},
	)
	output, planError := runCommand(t, server, "plan")
	if planError != nil {
		t.Fatalf("tael plan: %v", planError)
	}
	mustContain(t, output,
		"Plan:      launch\n",
		"Holding:   2 apps, 1 live · 0 open incidents · 1 decision on you\n",
		"Runtime:   ready (free) · addresses under tael.site\n",
		"Coupon:    none. Have one? `tael coupon <code>`.\n",
	)
	mustSpeakTael(t, output)

	output, couponError := runCommand(t, server, "coupon", "launch2026")
	if couponError != nil {
		t.Fatalf("tael coupon: %v", couponError)
	}
	mustContain(t, output, "Applied LAUNCH2026: this workspace is on launch until 2026-12-31 00:00 (3 apps, 1,000,000 AI tokens included).\n")
	if body := decodeBody(t, lastRequest(recorded, http.MethodPost, "/api/v1/workspace/coupon")); body["code"] != "launch2026" {
		t.Fatalf("coupon body = %v", body)
	}

	granted, _ := newAPIServer(t,
		route{http.MethodGet, "/api/v1/status", http.StatusOK, `{"workspace_status":"ready","plan":"launch","apps_total":0,"apps_live":0,"open_incidents":0,"needs_you":0,"runtime_status":null,"environment":null}`},
		route{http.MethodGet, "/api/v1/workspace/coupon", http.StatusOK, `{"grant":{"code":"LAUNCH2026","plan":"launch","until":"2026-12-31T00:00:00Z","apps_included":3,"ai_tokens_included":0}}`},
		route{http.MethodPost, "/api/v1/workspace/coupon", http.StatusConflict, `{"detail":"this workspace has already used a coupon"}`},
	)
	output, grantedError := runCommand(t, granted, "plan")
	if grantedError != nil || !strings.Contains(output, "Coupon:    LAUNCH2026 — this workspace is on launch until 2026-12-31 00:00 (3 apps included).\n") {
		t.Fatalf("tael plan with a coupon = %q, %v", output, grantedError)
	}
	_, refusal := runCommand(t, granted, "coupon", "AGAIN")
	if refusal == nil || !strings.Contains(refusal.Error(), "already used a coupon") {
		t.Fatalf("tael coupon twice = %v, want the 409 sentence", refusal)
	}
	unknown, _ := newAPIServer(t, route{http.MethodPost, "/api/v1/workspace/coupon", http.StatusNotFound, `{"detail":"that code is not one Tael knows, or it can no longer be claimed"}`})
	if _, unknownError := runCommand(t, unknown, "coupon", "NOPE"); unknownError == nil || !strings.Contains(unknownError.Error(), "not one Tael knows") {
		t.Fatalf("tael coupon NOPE = %v", unknownError)
	}
}
