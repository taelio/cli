package cmd

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/spf13/cobra"

	"tael.io/cli/internal/client"
)

var setupCmd = &cobra.Command{
	Use:   "setup [app]",
	Short: "Where an app's setup stands: what Tael read, what it wrote, the setup pull request",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(command *cobra.Command, args []string) error {
		appName, resolveError := resolveAppArgument(command, args)
		if resolveError != nil {
			return resolveError
		}
		setup, setupError := apiClient.GetAppSetup(command.Context(), appName)
		if setupError != nil {
			var apiError *client.APIError
			if errors.As(setupError, &apiError) && apiError.StatusCode == http.StatusBadGateway {
				// The runtime has nothing to say yet: Tael has not started on
				// the repository, or it is between steps. Its own sentence
				// would name what is underneath, so say it in Tael's words.
				return fmt.Errorf("Tael has not started reading %s yet, or is between steps. Try again in a moment", appName)
			}
			return setupError
		}
		if rendered, renderError := renderJSON(command, setup); rendered || renderError != nil {
			return renderError
		}
		renderSetup(command.OutOrStdout(), appName, setup)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(setupCmd)
}

// setupWords says where the setup is, in a phrase.
func setupWords(setup *client.AppSetup) string {
	switch {
	case setup.Status == "failed":
		return "stopped"
	case setup.PullRequestURL != "":
		return "the setup pull request is ready"
	default:
		return "Tael is reading the repository"
	}
}

func progressMark(status string) string {
	switch status {
	case "done", "completed", "succeeded", "success", "ok":
		return "✓"
	case "failed", "error":
		return "✗"
	}
	return "…"
}

// renderSetup prints the setup the way the onboarding screen narrates it:
// each step Tael took, what it detected, and what to do next.
func renderSetup(out io.Writer, appName string, setup *client.AppSetup) {
	fmt.Fprintf(out, "Setup for %s: %s.\n", appName, setupWords(setup))
	for _, entry := range setup.CreationProgress {
		message := strings.TrimSpace(entry.Message)
		if message == "" {
			continue
		}
		fmt.Fprintf(out, "  %s %s\n", progressMark(entry.Status), message)
	}
	if setup.DetectedFramework != "" {
		detected := setup.DetectedFramework
		if setup.DetectedLanguage != "" {
			detected += " (" + setup.DetectedLanguage + ")"
		}
		fmt.Fprintf(out, "Detected: %s\n", detected)
	}
	if len(setup.GeneratedFiles) > 0 {
		paths := make([]string, 0, len(setup.GeneratedFiles))
		for _, file := range setup.GeneratedFiles {
			paths = append(paths, file.Path)
		}
		fmt.Fprintf(out, "Written: %s\n", strings.Join(paths, ", "))
	}
	if setup.Status == "failed" {
		reason := strings.TrimSpace(setup.ErrorMessage)
		if reason == "" {
			reason = "Something in the setup did not work. The repository was not touched."
		}
		fmt.Fprintf(out, "Stopped: %s\n→ tael retry %s\n", reason, appName)
		return
	}
	if setup.PullRequestURL != "" {
		fmt.Fprintf(out, "Setup pull request: %s\n→ tael go-live %s\n", setup.PullRequestURL, appName)
	}
}
