package cmd

import (
	"fmt"
	"testing"
	"time"

	"tael.io/cli/internal/client"
)

func TestRenderAppsTable(t *testing.T) {
	previousLocal := time.Local
	time.Local = time.UTC
	t.Cleanup(func() { time.Local = previousLocal })

	apps := []client.App{
		{
			ID:           "app_1",
			Name:         "web",
			RepoFullName: "acme/web",
			Status:       "running",
			LiveURL:      "https://web.tael.app",
			UpdatedAt:    "2026-08-25T10:00:00Z",
		},
		{
			ID:           "app_2",
			Name:         "api-service",
			RepoFullName: "acme/api",
			Status:       "building",
			LiveURL:      "",
			UpdatedAt:    "2026-08-24T09:30:00Z",
		},
	}

	rendered := renderAppsTable(apps)

	expected := "" +
		fmt.Sprintf("%-13s%-10s%-22s%s\n", "NAME", "STATUS", "URL", "UPDATED") +
		fmt.Sprintf("%-13s%-10s%-22s%s\n", "web", "running", "https://web.tael.app", "2026-08-25 10:00") +
		fmt.Sprintf("%-13s%-10s%-22s%s\n", "api-service", "building", "-", "2026-08-24 09:30")

	if rendered != expected {
		t.Fatalf("renderAppsTable mismatch\ngot:\n%s\nwant:\n%s", rendered, expected)
	}
}

func TestFormatTimestampFallsBackToRawValue(t *testing.T) {
	if formatted := formatTimestamp("just now"); formatted != "just now" {
		t.Fatalf("formatTimestamp = %q, want the raw string back", formatted)
	}
}
