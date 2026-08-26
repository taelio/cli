package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status [app]",
	Short: "Show an app's live status and health checks",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(command *cobra.Command, args []string) error {
		appName, resolveError := resolveAppArgument(command, args)
		if resolveError != nil {
			return resolveError
		}
		statusResponse, statusError := apiClient.GetAppStatus(command.Context(), appName)
		if statusError != nil {
			return statusError
		}
		if rendered, renderError := renderJSON(command, statusResponse); rendered || renderError != nil {
			return renderError
		}

		out := command.OutOrStdout()
		fmt.Fprintf(out, "App:     %s\n", appName)
		fmt.Fprintf(out, "Status:  %s\n", statusResponse.Status)
		fmt.Fprintf(out, "Healthy: %t\n", statusResponse.Healthy)
		if statusResponse.LiveURL != "" {
			fmt.Fprintf(out, "URL:     %s\n", statusResponse.LiveURL)
		}
		if len(statusResponse.Checks) > 0 {
			fmt.Fprintln(out)
			fmt.Fprintln(out, "Checks:")
			for _, check := range statusResponse.Checks {
				line := fmt.Sprintf("  %-8s %s", check.Status, check.Name)
				if check.Message != "" {
					line += " — " + check.Message
				}
				fmt.Fprintln(out, line)
			}
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(statusCmd)
}
