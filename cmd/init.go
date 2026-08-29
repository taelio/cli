package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "How to connect a repository to Tael",
	Args:  cobra.NoArgs,
	RunE: func(command *cobra.Command, _ []string) error {
		out := command.OutOrStdout()
		fmt.Fprintln(out, "Putting a repository live takes two steps.")
		fmt.Fprintln(out)
		fmt.Fprintln(out, "  1. Let Tael see your code. Installing the Tael GitHub App is a browser step:")
		fmt.Fprintln(out, "     open the web app, choose New app, and pick the repositories.")
		fmt.Fprintln(out, "  2. Put one live from here:")
		fmt.Fprintln(out)
		fmt.Fprintln(out, "       tael repos")
		fmt.Fprintln(out, "       tael new --repo owner/name")
		fmt.Fprintln(out)
		fmt.Fprintln(out, "Tael reads the repository, sets the app up and opens a setup pull request;")
		fmt.Fprintln(out, "`tael go-live <app>` merges it and the first deploy starts.")
		return nil
	},
}

func init() {
	setRequiresAuth(initCmd, false)
	rootCmd.AddCommand(initCmd)
}
