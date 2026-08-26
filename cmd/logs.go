package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var followLogs bool

var logsCmd = &cobra.Command{
	Use:   "logs [app]",
	Short: "Print an app's logs",
	Long: `Print recent log lines for an app.

With -f/--follow the CLI stays attached and streams new lines as the
platform emits them; interrupt with Ctrl+C.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(command *cobra.Command, args []string) error {
		appName, resolveError := resolveAppArgument(command, args)
		if resolveError != nil {
			return resolveError
		}
		out := command.OutOrStdout()

		if followLogs {
			return apiClient.FollowLogs(command.Context(), appName, func(line string) {
				fmt.Fprintln(out, line)
			})
		}

		logsResponse, logsError := apiClient.GetLogs(command.Context(), appName)
		if logsError != nil {
			return logsError
		}
		if rendered, renderError := renderJSON(command, logsResponse); rendered || renderError != nil {
			return renderError
		}
		for _, line := range logsResponse.Lines {
			fmt.Fprintln(out, line)
		}
		return nil
	},
}

func init() {
	logsCmd.Flags().BoolVarP(&followLogs, "follow", "f", false, "stream new log lines as they arrive")
	rootCmd.AddCommand(logsCmd)
}
