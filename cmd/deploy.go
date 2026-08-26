package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var deployCmd = &cobra.Command{
	Use:   "deploy [app]",
	Short: "Trigger a deploy of an app",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(command *cobra.Command, args []string) error {
		appName, resolveError := resolveAppArgument(command, args)
		if resolveError != nil {
			return resolveError
		}
		createResponse, createError := apiClient.CreateDeploy(command.Context(), appName)
		if createError != nil {
			return createError
		}
		if rendered, renderError := renderJSON(command, createResponse); rendered || renderError != nil {
			return renderError
		}
		out := command.OutOrStdout()
		fmt.Fprintf(out, "Deploy %s started for %s.\n", createResponse.DeployID, appName)
		fmt.Fprintf(out, "Follow it with `tael logs %s -f`.\n", appName)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(deployCmd)
}
