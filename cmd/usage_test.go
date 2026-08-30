package cmd

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestUsage(t *testing.T) {
	server, recorded := newAPIServer(t,
		route{http.MethodGet, "/api/v1/usage", http.StatusOK, `{"period":"2026-08-01","ai_tokens":{"used":1200000,"included":20000000,"source":"coupon","estimated":150000},"apps":{"used":3,"included":5},"seats":{"used":1,"included":3},"custom_domains":{"allowed":true,"used":1},"deploys":14,"builds":9,"machines":2}`},
	)
	output, usageError := runCommand(t, server, "usage")
	if usageError != nil {
		t.Fatalf("tael usage: %v", usageError)
	}
	want := "Period      August 2026\n" +
		"Apps        3 of 5\n" +
		"Seats       1 of 3\n" +
		"AI tokens   1.2M of 20M (coupon) · part estimated\n" +
		"Custom domains  yes · 1 in use\n" +
		"Deploys     14 this period\n" +
		"Builds      9 this period\n" +
		"Machines    2\n"
	if output != want {
		t.Fatalf("tael usage =\n%s\nwant:\n%s", output, want)
	}
	mustSpeakTael(t, output)
	if request := lastRequest(recorded, http.MethodGet, "/api/v1/usage"); request == nil {
		t.Fatalf("no GET /api/v1/usage recorded")
	}

	// -o json prints the API's answer as it came.
	jsonOutput, jsonError := runCommand(t, server, "usage", "-o", "json")
	if jsonError != nil {
		t.Fatalf("tael usage -o json: %v", jsonError)
	}
	var decoded map[string]any
	if unmarshalError := json.Unmarshal([]byte(jsonOutput), &decoded); unmarshalError != nil {
		t.Fatalf("tael usage -o json is not JSON: %v\n%s", unmarshalError, jsonOutput)
	}
	if decoded["period"] != "2026-08-01" || decoded["ai_tokens"].(map[string]any)["estimated"] != float64(150000) {
		t.Fatalf("tael usage -o json = %v", decoded)
	}
}

func TestUsageOnTheFreePlan(t *testing.T) {
	// Reported tokens only, an allowance from the plan, custom domains not
	// yet open: no coupon note, no estimated hint, "on Launch".
	server, _ := newAPIServer(t,
		route{http.MethodGet, "/api/v1/usage", http.StatusOK, `{"period":"2026-08-01T00:00:00Z","ai_tokens":{"used":850,"included":200000,"source":"plan","estimated":0},"apps":{"used":5,"included":5},"seats":{"used":3,"included":3},"custom_domains":{"allowed":false,"used":0},"deploys":1,"builds":0,"machines":0}`},
	)
	output, usageError := runCommand(t, server, "usage")
	if usageError != nil {
		t.Fatalf("tael usage: %v", usageError)
	}
	want := "Period      August 2026\n" +
		"Apps        5 of 5\n" +
		"Seats       3 of 3\n" +
		"AI tokens   850 of 200k\n" +
		"Custom domains  on Launch\n" +
		"Deploys     1 this period\n" +
		"Builds      0 this period\n"
	if output != want {
		t.Fatalf("tael usage =\n%s\nwant:\n%s", output, want)
	}
	mustSpeakTael(t, output)
}

func TestUsageTolerantOfMissingFields(t *testing.T) {
	// An API that keeps fewer meters: only what came is shown, an allowance
	// of nothing is no allowance, and a paid plan with no custom domain in
	// use says "yes".
	server, _ := newAPIServer(t,
		route{http.MethodGet, "/api/v1/usage", http.StatusOK, `{"apps":{"used":2},"custom_domains":{"allowed":true},"deploys":3}`},
	)
	output, usageError := runCommand(t, server, "usage")
	if usageError != nil {
		t.Fatalf("tael usage: %v", usageError)
	}
	want := "Apps        2\n" +
		"Custom domains  yes\n" +
		"Deploys     3 this period\n"
	if output != want {
		t.Fatalf("tael usage =\n%s\nwant:\n%s", output, want)
	}

	empty, _ := newAPIServer(t, route{http.MethodGet, "/api/v1/usage", http.StatusOK, `{}`})
	output, emptyError := runCommand(t, empty, "usage")
	if emptyError != nil || output != "Nothing counted yet this period.\n" {
		t.Fatalf("tael usage on nothing = %q, %v", output, emptyError)
	}
}

func TestCompactCount(t *testing.T) {
	cases := map[int64]string{0: "0", 850: "850", 1000: "1k", 1500: "1.5k", 200000: "200k", 1200000: "1.2M", 5000000: "5M", 20000000: "20M", 25000000: "25M"}
	for value, want := range cases {
		if got := compactCount(value); got != want {
			t.Errorf("compactCount(%d) = %q, want %q", value, got, want)
		}
	}
}
