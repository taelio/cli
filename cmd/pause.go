package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"tael.io/cli/internal/client"
)

var pauseCmd = &cobra.Command{
	Use:   "pause",
	Short: "Stop Tael starting or carrying out anything until you resume",
	Long: `Pause Tael for the whole workspace: nothing new starts and nothing
waiting is carried out until ` + "`tael resume`" + `. Tael keeps watching, so
Activity stays current. Owners and admins only.`,
	Args: cobra.NoArgs,
	RunE: func(command *cobra.Command, _ []string) error {
		settings, saveError := apiClient.SetPaused(command.Context(), true)
		if saveError != nil {
			return saveError
		}
		if rendered, renderError := renderJSON(command, settings); rendered || renderError != nil {
			return renderError
		}
		fmt.Fprint(command.OutOrStdout(), renderPauseState(settings))
		return nil
	},
}

var resumeCmd = &cobra.Command{
	Use:   "resume",
	Short: "Let Tael work again after a pause",
	Args:  cobra.NoArgs,
	RunE: func(command *cobra.Command, _ []string) error {
		settings, saveError := apiClient.SetPaused(command.Context(), false)
		if saveError != nil {
			return saveError
		}
		if rendered, renderError := renderJSON(command, settings); rendered || renderError != nil {
			return renderError
		}
		fmt.Fprint(command.OutOrStdout(), renderPauseState(settings))
		return nil
	},
}

func init() {
	rootCmd.AddCommand(pauseCmd)
	rootCmd.AddCommand(resumeCmd)
}

// renderPauseState says which way Tael is set after a pause or resume.
func renderPauseState(settings *client.AISettings) string {
	if settings.Paused {
		return "Tael is paused. Nothing new starts and nothing waiting is carried out until `tael resume`.\n"
	}
	return "Tael is watching again. It asks before it changes anything that is running.\n"
}
