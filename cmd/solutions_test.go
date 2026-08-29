package cmd

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"tael.io/cli/internal/client"
)

func TestRenderSolutionsTable(t *testing.T) {
	previousLocal := time.Local
	time.Local = time.UTC
	t.Cleanup(func() { time.Local = previousLocal })

	solutions := []client.Solution{
		{
			ID:          "sol_1",
			SolutionKey: "postgres",
			Name:        "Tael Managed Postgres for web",
			PresetLabel: "Small",
			Status:      "ready",
			App:         &client.SolutionApp{ID: "app_1", Name: "web"},
			Bindings:    []client.SolutionBinding{{AppID: "app_1", AppName: "web", Status: "bound"}},
			InstalledAt: "2026-08-26T10:00:00Z",
		},
		{
			ID:              "sol_2",
			SolutionKey:     "monitoring",
			Name:            "Tael Managed Monitoring",
			PresetLabel:     "Small",
			Status:          "degraded",
			UpdateAvailable: true,
			InstalledAt:     "",
		},
	}

	rendered := renderSolutionsTable(solutions)

	expected := "" +
		fmt.Sprintf("%-31s%-29s%-7s%-5s%-11s%s\n", "NAME", "STATUS", "SIZE", "FOR", "CONNECTED", "INSTALLED") +
		fmt.Sprintf("%-31s%-29s%-7s%-5s%-11s%s\n", "Tael Managed Postgres for web", "ready", "Small", "web", "web", "2026-08-26 10:00") +
		fmt.Sprintf("%-31s%-29s%-7s%-5s%-11s%s\n", "Tael Managed Monitoring", "degraded (update available)", "Small", "-", "-", "-")

	if rendered != expected {
		t.Fatalf("renderSolutionsTable mismatch\ngot:\n%s\nwant:\n%s", rendered, expected)
	}
}

func TestPresentSolutionsDropsWhatIsGone(t *testing.T) {
	present := presentSolutions([]client.Solution{
		{ID: "a", Status: "ready"},
		{ID: "b", Status: "removed"},
		{ID: "c", Status: "retired"},
		{ID: "d", Status: "provisioning"},
	})
	if len(present) != 2 || present[0].ID != "a" || present[1].ID != "d" {
		t.Fatalf("presentSolutions = %+v, want a and d", present)
	}
}

func TestRenderSolutionStatus(t *testing.T) {
	solution := &client.Solution{
		Name:        "Tael Managed Postgres for web",
		PresetLabel: "Small",
		App:         &client.SolutionApp{Name: "web"},
		Bindings:    []client.SolutionBinding{{AppName: "web"}, {AppName: "api"}},
	}
	status := &client.SolutionStatusResponse{
		Status:  "degraded",
		Healthy: false,
		Checks: []client.HealthCheck{
			{Name: "solution", Status: "failed", Message: "Tael Managed Postgres for web is running but not healthy. Ask Tael why."},
		},
		Pods: []client.PodCheck{{Name: "tael-postgres-web-1", Phase: "Running", Ready: "0/1", Restarts: 3}},
	}

	rendered := renderSolutionStatus(solution, status)

	expected := strings.Join([]string{
		"Solution: Tael Managed Postgres for web",
		"Size:     Small",
		"Status:   degraded",
		"Healthy:  false",
		"For:      web",
		"Apps:     web, api",
		"",
		"Checks:",
		"  failed   solution — Tael Managed Postgres for web is running but not healthy. Ask Tael why.",
		"",
		"Pods:",
		"  Running    tael-postgres-web-1 0/1 ready, 3 restarts",
		"",
	}, "\n")
	if rendered != expected {
		t.Fatalf("renderSolutionStatus mismatch\ngot:\n%s\nwant:\n%s", rendered, expected)
	}
}

func TestRenderCatalogHintShowsThePlanGate(t *testing.T) {
	rendered := renderCatalogHint([]client.CatalogEntry{
		{Key: "postgres", Name: "Tael Managed Postgres", Availability: client.SolutionAvailability{State: "available", Label: "Add"}},
		{Key: "monitoring", Name: "Tael Managed Monitoring", Availability: client.SolutionAvailability{State: "plan_required", Label: "Available on Launch"}},
	})
	if !strings.HasPrefix(rendered, "Add one with `tael solutions add <key>`:\n") {
		t.Fatalf("hint = %q, want the add instruction first", rendered)
	}
	if !strings.Contains(rendered, "postgres") || !strings.Contains(rendered, "Available on Launch") {
		t.Fatalf("hint = %q, want the keys and the plan gate", rendered)
	}
	if strings.Contains(rendered, "Add\n") {
		t.Fatalf("hint = %q, want no label on a row that can simply be added", rendered)
	}
}

// The CLI speaks Tael's vocabulary: nothing it prints names the platform
// underneath or what a solution is built from.
func TestSolutionsCommandsNeverSayThePlatform(t *testing.T) {
	texts := []string{
		solutionsCmd.Short, solutionsCmd.Long,
		solutionsListCmd.Short, solutionsAddCmd.Short, solutionsRemoveCmd.Short,
		solutionsStatusCmd.Short, solutionsConnectCmd.Short,
		solutionsAddCmd.Flags().Lookup("for").Usage,
		solutionsAddCmd.Flags().Lookup("size").Usage,
		solutionsRemoveCmd.Flags().Lookup("force").Usage,
		solutionsConnectCmd.Flags().Lookup("app").Usage,
		renderSolutionsTable([]client.Solution{{Name: "Tael Managed Postgres", Status: "ready"}}),
	}
	for _, text := range texts {
		lower := strings.ToLower(text)
		for _, word := range []string{"ankra", "stack", "profile", "helm", "addon"} {
			if strings.Contains(lower, word) {
				t.Fatalf("%q says %q", text, word)
			}
		}
	}
}
