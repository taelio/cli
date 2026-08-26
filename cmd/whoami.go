package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var whoamiCmd = &cobra.Command{
	Use:   "whoami",
	Short: "Show the authenticated user and workspace",
	Args:  cobra.NoArgs,
	RunE: func(command *cobra.Command, _ []string) error {
		whoamiResponse, whoamiError := apiClient.Whoami(command.Context())
		if whoamiError != nil {
			return whoamiError
		}
		if rendered, renderError := renderJSON(command, whoamiResponse); rendered || renderError != nil {
			return renderError
		}

		out := command.OutOrStdout()
		fmt.Fprintf(out, "User:      %s (@%s)\n", whoamiResponse.User.Name, whoamiResponse.User.GithubLogin)
		fmt.Fprintf(out, "Workspace: %s (%s)\n", whoamiResponse.Workspace.Name, whoamiResponse.Workspace.Slug)
		fmt.Fprintf(out, "Plan:      %s\n", whoamiResponse.Workspace.Plan)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(whoamiCmd)
}
