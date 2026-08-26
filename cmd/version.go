package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// version and commit are injected at build time via
// -ldflags "-X tael.io/cli/cmd.version=... -X tael.io/cli/cmd.commit=...".
var (
	version = "dev"
	commit  = "none"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show the CLI version",
	Args:  cobra.NoArgs,
	RunE: func(command *cobra.Command, _ []string) error {
		if rendered, renderError := renderJSON(command, map[string]string{
			"version": version,
			"commit":  commit,
		}); rendered || renderError != nil {
			return renderError
		}
		fmt.Fprintf(command.OutOrStdout(), "tael %s (commit %s)\n", version, commit)
		return nil
	},
}

func init() {
	setRequiresAuth(versionCmd, false)
	rootCmd.AddCommand(versionCmd)
}
