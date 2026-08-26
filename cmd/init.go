package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Connect a repository to tael",
	Args:  cobra.NoArgs,
	RunE: func(command *cobra.Command, _ []string) error {
		out := command.OutOrStdout()
		fmt.Fprintln(out, "Connecting a repository from the CLI is not available yet.")
		fmt.Fprintln(out)
		fmt.Fprintln(out, "Connect it from the web app instead:")
		fmt.Fprintln(out)
		fmt.Fprintln(out, "  https://tael.io/app/new")
		fmt.Fprintln(out)
		fmt.Fprintln(out, "Once connected, `tael apps` will show it here.")
		return nil
	},
}

func init() {
	setRequiresAuth(initCmd, false)
	rootCmd.AddCommand(initCmd)
}
