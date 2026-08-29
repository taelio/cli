package cmd

import (
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"tael.io/cli/internal/client"
)

var appCmd = &cobra.Command{
	Use:   "app [app]",
	Short: "Show one app: status, address, repository, framework, last deploy and checks",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(command *cobra.Command, args []string) error {
		appName, resolveError := resolveAppArgument(command, args)
		if resolveError != nil {
			return resolveError
		}
		detail, detailError := apiClient.GetApp(command.Context(), appName)
		if detailError != nil {
			return detailError
		}
		status, statusError := apiClient.GetAppStatus(command.Context(), appName)
		if statusError != nil {
			return statusError
		}
		if rendered, renderError := renderJSON(command, map[string]any{"app": detail, "status": status}); rendered || renderError != nil {
			return renderError
		}
		renderAppDetail(command.OutOrStdout(), detail, status)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(appCmd)
}

// renderAppDetail prints an app the way its page reads: the facts, then
// the last deploy, then the checks.
func renderAppDetail(out io.Writer, detail *client.AppDetail, status *client.AppStatusResponse) {
	health := "not healthy"
	if status.Healthy {
		health = "healthy"
	}
	fmt.Fprintf(out, "App:       %s\n", detail.Name)
	fmt.Fprintf(out, "Status:    %s (%s)\n", valueOrDash(status.Status), health)
	fmt.Fprintf(out, "Stage:     %s\n", valueOrDash(strings.ReplaceAll(detail.PipelineStage, "_", " ")))
	if url := firstNonEmpty(status.LiveURL, detail.LiveURL); url != "" {
		fmt.Fprintf(out, "Address:   %s\n", url)
	}
	fmt.Fprintf(out, "Repo:      %s\n", valueOrDash(detail.RepoFullName))
	if detail.DetectedFramework != "" {
		fmt.Fprintf(out, "Framework: %s\n", detail.DetectedFramework)
	}
	if detail.DetectedGenerator != "" {
		fmt.Fprintf(out, "Made with: %s\n", detail.DetectedGenerator)
	}
	if deploy := detail.LastDeploy; deploy != nil {
		when := deploy.FinishedAt
		if when == "" {
			when = deploy.CreatedAt
		}
		line := fmt.Sprintf("Last deploy: %s", valueOrDash(deploy.Status))
		if commit := shortCommit(deploy.CommitSHA); commit != "" {
			line += " · " + commit
		}
		if message := firstLine(deploy.CommitMessage); message != "" {
			line += " " + strings.TrimSpace(message)
		}
		if formatted := formatTimestamp(when); formatted != "" {
			line += " · " + formatted
		}
		fmt.Fprintln(out, line)
	}
	if len(status.Checks) > 0 {
		fmt.Fprintln(out)
		fmt.Fprintln(out, "Checks:")
		for _, check := range status.Checks {
			line := fmt.Sprintf("  %-8s %s", check.Status, check.Name)
			if check.Message != "" {
				line += " — " + check.Message
			}
			fmt.Fprintln(out, line)
		}
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
