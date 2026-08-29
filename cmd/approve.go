package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"tael.io/cli/internal/client"
)

var decisionNote string

var approveCmd = &cobra.Command{
	Use:   "approve <task-id>",
	Short: "Say yes to what Tael is waiting on",
	Long: `Approve the decision a task is waiting on — or a proposal as a whole —
so Tael carries it out. A note is kept with the decision for the record.`,
	Args: cobra.ExactArgs(1),
	RunE: func(command *cobra.Command, args []string) error {
		detail, decideError := apiClient.ApproveTask(command.Context(), args[0], decisionNote)
		if decideError != nil {
			return decideError
		}
		if rendered, renderError := renderJSON(command, detail); rendered || renderError != nil {
			return renderError
		}
		fmt.Fprint(command.OutOrStdout(), renderDecision(detail, true))
		return nil
	},
}

var declineCmd = &cobra.Command{
	Use:   "decline <task-id>",
	Short: "Say no to what Tael is waiting on",
	Args:  cobra.ExactArgs(1),
	RunE: func(command *cobra.Command, args []string) error {
		detail, decideError := apiClient.DeclineTask(command.Context(), args[0], decisionNote)
		if decideError != nil {
			return decideError
		}
		if rendered, renderError := renderJSON(command, detail); rendered || renderError != nil {
			return renderError
		}
		fmt.Fprint(command.OutOrStdout(), renderDecision(detail, false))
		return nil
	},
}

func init() {
	approveCmd.Flags().StringVar(&decisionNote, "note", "", "a note kept with the decision")
	declineCmd.Flags().StringVar(&decisionNote, "note", "", "a note kept with the decision")
	rootCmd.AddCommand(approveCmd)
	rootCmd.AddCommand(declineCmd)
}

// renderDecision says what happened after a decision, and what to do next.
func renderDecision(detail *client.TaskDetail, approved bool) string {
	task := detail.Task
	if approved {
		return fmt.Sprintf("Approved: %s\nTael is on it (%s). Follow it with `tael task %s`.\n",
			task.Title, statusWords(task.Status), task.ID)
	}
	return fmt.Sprintf("Declined: %s\nTael won't do that; the task keeps the record (%s).\n",
		task.Title, statusWords(task.Status))
}
