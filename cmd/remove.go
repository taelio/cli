package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var removeYesFlag bool

var removeCmd = &cobra.Command{
	Use:   "remove <app>",
	Short: "Take an app out of Tael; the repository is untouched",
	Long: `Take an app out of Tael: it stops being deployed and leaves the list. Open
work on it is cancelled. The repository itself is untouched. Say --yes to
confirm.`,
	Args: cobra.ExactArgs(1),
	RunE: func(command *cobra.Command, args []string) error {
		if !removeYesFlag {
			return withExitCode(exitUsage, fmt.Errorf(
				"this takes %s out of Tael (the repository is untouched); run again with --yes to confirm", args[0]))
		}
		response, removeError := apiClient.RemoveApp(command.Context(), args[0])
		if removeError != nil {
			return removeError
		}
		if rendered, renderError := renderJSON(command, response); rendered || renderError != nil {
			return renderError
		}
		fmt.Fprintf(command.OutOrStdout(), "Removed %s from Tael. The repository is untouched.\n", args[0])
		return nil
	},
}

var retryCmd = &cobra.Command{
	Use:   "retry <app>",
	Short: "Run a failed setup again from the step that failed",
	Args:  cobra.ExactArgs(1),
	RunE: func(command *cobra.Command, args []string) error {
		response, retryError := apiClient.RetryApp(command.Context(), args[0])
		if retryError != nil {
			return retryError
		}
		if rendered, renderError := renderJSON(command, response); rendered || renderError != nil {
			return renderError
		}
		fmt.Fprintf(command.OutOrStdout(), "Setting up %s again from where it stopped. Follow it with `tael setup %s`.\n", args[0], args[0])
		return nil
	},
}

var goLiveCmd = &cobra.Command{
	Use:   "go-live <app>",
	Short: "Merge the setup pull request so the first deploy starts",
	Args:  cobra.ExactArgs(1),
	RunE: func(command *cobra.Command, args []string) error {
		response, goLiveError := apiClient.GoLive(command.Context(), args[0])
		if goLiveError != nil {
			return goLiveError
		}
		if rendered, renderError := renderJSON(command, response); rendered || renderError != nil {
			return renderError
		}
		out := command.OutOrStdout()
		fmt.Fprintf(out, "%s is going live: the setup pull request is merged and the first deploy is on its way.\n", args[0])
		if response.PullRequestURL != "" {
			fmt.Fprintf(out, "Merged: %s\n", response.PullRequestURL)
		}
		fmt.Fprintf(out, "Follow it with `tael logs %s -f`.\n", args[0])
		return nil
	},
}

func init() {
	removeCmd.Flags().BoolVar(&removeYesFlag, "yes", false, "confirm taking the app out of Tael")
	rootCmd.AddCommand(removeCmd, retryCmd, goLiveCmd)
}
