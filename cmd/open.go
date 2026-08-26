package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var openCmd = &cobra.Command{
	Use:   "open [app]",
	Short: "Open an app's live URL in the browser",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(command *cobra.Command, args []string) error {
		appName, resolveError := resolveAppArgument(command, args)
		if resolveError != nil {
			return resolveError
		}
		appDetail, appError := apiClient.GetApp(command.Context(), appName)
		if appError != nil {
			return appError
		}
		if appDetail.LiveURL == "" {
			return fmt.Errorf("%s has no live URL yet (status: %s)", appDetail.Name, appDetail.Status)
		}
		fmt.Fprintf(command.OutOrStdout(), "Opening %s\n", appDetail.LiveURL)
		if browserError := openBrowser(appDetail.LiveURL); browserError != nil {
			return fmt.Errorf("open browser: %w", browserError)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(openCmd)
}
