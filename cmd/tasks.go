package cmd

import (
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"tael.io/cli/internal/client"
)

var tasksDone bool

var tasksCmd = &cobra.Command{
	Use:   "tasks [app]",
	Short: "List what Tael is doing, has done, or needs you for",
	Long: `List the workspace's tasks: every deploy, investigation, proposal and
install, newest first, with anything that needs your decision on top.

By default only open tasks are shown; --done lists finished ones. Name an
app to narrow the list to it.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(command *cobra.Command, args []string) error {
		app := ""
		if len(args) > 0 {
			app = args[0]
		}
		status := "open"
		if tasksDone {
			status = "done"
		}
		listResponse, listError := apiClient.ListTasks(command.Context(), status, app)
		if listError != nil {
			return listError
		}
		if rendered, renderError := renderJSON(command, listResponse); rendered || renderError != nil {
			return renderError
		}
		out := command.OutOrStdout()
		if len(listResponse.Tasks) == 0 {
			if tasksDone {
				fmt.Fprintln(out, "Nothing finished yet.")
			} else {
				fmt.Fprintln(out, "Quiet. Tasks appear here as Tael works.")
			}
			return nil
		}
		fmt.Fprint(out, renderTasksTable(listResponse.Tasks))
		return nil
	},
}

var taskCmd = &cobra.Command{
	Use:   "task <id>",
	Short: "Show one task: its plan, evidence and outcome",
	Args:  cobra.ExactArgs(1),
	RunE: func(command *cobra.Command, args []string) error {
		detail, getError := apiClient.GetTask(command.Context(), args[0])
		if getError != nil {
			return getError
		}
		if rendered, renderError := renderJSON(command, detail); rendered || renderError != nil {
			return renderError
		}
		renderTaskDetail(command.OutOrStdout(), detail)
		return nil
	},
}

var (
	tasksNewAppFlag  string
	tasksNewKindFlag string
)

var tasksNewCmd = &cobra.Command{
	Use:   "new <brief>",
	Short: "Ask Tael to look into something, in your words",
	Long: `Open a task in your words: "why is the checkout slow?", "review the
last deploy". Name an app with --app; --kind is investigate (the default),
fix, review or audit.`,
	Args: cobra.ExactArgs(1),
	RunE: func(command *cobra.Command, args []string) error {
		brief := strings.TrimSpace(args[0])
		if brief == "" {
			return withExitCode(exitUsage, fmt.Errorf("say what Tael should look into"))
		}
		detail, createError := apiClient.CreateTask(command.Context(), brief, tasksNewAppFlag, tasksNewKindFlag)
		if createError != nil {
			return createError
		}
		if rendered, renderError := renderJSON(command, detail); rendered || renderError != nil {
			return renderError
		}
		fmt.Fprint(command.OutOrStdout(), renderTaskOpened(detail))
		return nil
	},
}

var taskCommentCmd = &cobra.Command{
	Use:   "comment <id> <text>",
	Short: "Leave a comment on a task, for Tael and the team",
	Args:  cobra.ExactArgs(2),
	RunE: func(command *cobra.Command, args []string) error {
		comment, commentError := apiClient.AddTaskComment(command.Context(), args[0], args[1])
		if commentError != nil {
			return commentError
		}
		if rendered, renderError := renderJSON(command, comment); rendered || renderError != nil {
			return renderError
		}
		fmt.Fprintf(command.OutOrStdout(), "Noted on the task: %s\n", strings.TrimSpace(comment.Body))
		return nil
	},
}

var needsYouCmd = &cobra.Command{
	Use:   "needs-you",
	Short: "What is waiting on you: decisions Tael has asked for, and proposals",
	Args:  cobra.NoArgs,
	RunE: func(command *cobra.Command, _ []string) error {
		response, listError := apiClient.NeedsYou(command.Context())
		if listError != nil {
			return listError
		}
		if rendered, renderError := renderJSON(command, response); rendered || renderError != nil {
			return renderError
		}
		fmt.Fprint(command.OutOrStdout(), renderNeedsYou(response))
		return nil
	},
}

func init() {
	tasksCmd.Flags().BoolVar(&tasksDone, "done", false, "list finished tasks instead of open ones")
	tasksNewCmd.Flags().StringVar(&tasksNewAppFlag, "app", "", "the app it is about (name or id)")
	tasksNewCmd.Flags().StringVar(&tasksNewKindFlag, "kind", "", "investigate (the default), fix, review or audit")
	tasksCmd.AddCommand(tasksNewCmd)
	taskCmd.AddCommand(taskCommentCmd)
	rootCmd.AddCommand(tasksCmd)
	rootCmd.AddCommand(taskCmd)
	rootCmd.AddCommand(needsYouCmd)
}

// renderTaskOpened says what happened to a request, the way `tael why`
// does: on it, or on the record when this workspace cannot run it.
func renderTaskOpened(detail *client.TaskDetail) string {
	task := detail.Task
	if task.Status == "proposed" {
		return fmt.Sprintf("Asked Tael: %s\nTael can't run that on this workspace yet; the task is on the record: tael task %s\n", task.Title, task.ID)
	}
	return fmt.Sprintf("Asked Tael: %s\nTael is on it (%s). Follow it with `tael task %s`.\n", task.Title, statusWords(task.Status), task.ID)
}

// renderNeedsYou renders the strip in text: each decision with its task,
// and what to type.
func renderNeedsYou(response *client.NeedsYouResponse) string {
	if len(response.Items) == 0 {
		return "Nothing needs you. Tael says so here when something does.\n"
	}
	var builder strings.Builder
	fmt.Fprintf(&builder, "Needs you (%d):\n", len(response.Items))
	for _, item := range response.Items {
		task := item.Task
		line := "  " + task.Title
		if item.Approval != nil && strings.TrimSpace(item.Approval.Summary) != "" {
			line += " — " + strings.TrimSpace(item.Approval.Summary)
			var facts []string
			if item.Approval.Risk != "" {
				facts = append(facts, item.Approval.Risk+" risk")
			}
			if item.Approval.Reversible {
				facts = append(facts, "reversible")
			} else if item.Approval.Risk != "" {
				facts = append(facts, "not reversible")
			}
			if len(facts) > 0 {
				line += " (" + strings.Join(facts, ", ") + ")"
			}
		} else if task.Status == "proposed" {
			line += " — Tael proposes this"
		}
		if name := appName(task); name != "" {
			line += " · " + name
		}
		waitingSince := task.CreatedAt
		if item.Approval != nil && item.Approval.RequestedAt != "" {
			waitingSince = item.Approval.RequestedAt
		}
		if formatted := formatTimestamp(waitingSince); formatted != "" {
			line += " · since " + formatted
		}
		builder.WriteString(line + "\n")
		fmt.Fprintf(&builder, "  → tael approve %s   or   tael decline %s\n", task.ID, task.ID)
	}
	return builder.String()
}

// statusWords turns an API status into words: needs_approval → "needs you",
// waiting_approval → "waiting for you", the rest with underscores as spaces.
func statusWords(status string) string {
	switch status {
	case "needs_approval":
		return "needs you"
	case "waiting_approval":
		return "waiting for you"
	case "":
		return "-"
	}
	return strings.ReplaceAll(status, "_", " ")
}

// kindWords names a task's kind the way a person would.
func kindWords(kind string) string {
	switch kind {
	case "investigate":
		return "investigation"
	case "":
		return "-"
	}
	return kind
}

func appName(task client.Task) string {
	if task.App == nil {
		return ""
	}
	return task.App.Name
}

// taskWhen is the time a row is dated by: the finish when there is one,
// otherwise when it opened.
func taskWhen(task client.Task) string {
	if task.FinishedAt != nil && *task.FinishedAt != "" {
		return formatTimestamp(*task.FinishedAt)
	}
	return formatTimestamp(task.CreatedAt)
}

// renderTasksTable renders the tasks list as an aligned text table.
func renderTasksTable(tasks []client.Task) string {
	var builder strings.Builder
	table := tabwriter.NewWriter(&builder, 0, 0, 2, ' ', 0)
	fmt.Fprintln(table, "ID\tSTATUS\tKIND\tTITLE\tAPP\tWHEN")
	for _, task := range tasks {
		status := statusWords(task.Status)
		if task.NeedsYou && task.Status != "needs_approval" {
			status += " · needs you"
		}
		fmt.Fprintf(table, "%s\t%s\t%s\t%s\t%s\t%s\n",
			task.ID,
			status,
			kindWords(task.Kind),
			valueOrDash(task.Title),
			valueOrDash(appName(task)),
			valueOrDash(taskWhen(task)),
		)
	}
	_ = table.Flush()
	return builder.String()
}

func okMark(ok *bool) string {
	switch {
	case ok == nil:
		return "·"
	case *ok:
		return "✓"
	default:
		return "✗"
	}
}

// transcriptLines splits a transcript body into its narrated lines,
// dropping blanks and leading bullets.
func transcriptLines(body string) []string {
	var lines []string
	for _, raw := range strings.Split(body, "\n") {
		line := strings.TrimSpace(raw)
		line = strings.TrimLeft(line, "-*• ")
		if line == "" {
			continue
		}
		lines = append(lines, line)
	}
	return lines
}

func indentBlock(body string, prefix string, maxLines int) string {
	lines := strings.Split(strings.TrimRight(body, "\n"), "\n")
	if maxLines > 0 && len(lines) > maxLines {
		lines = append(lines[:maxLines], fmt.Sprintf("… %d more lines", len(lines)-maxLines))
	}
	for index, line := range lines {
		lines[index] = prefix + line
	}
	return strings.Join(lines, "\n")
}

// renderTaskDetail prints a task the way its page reads: the header, then
// Plan, Change, Evidence, Outcome and Comments, each only when it has
// something to say.
func renderTaskDetail(out io.Writer, detail *client.TaskDetail) {
	task := detail.Task
	fmt.Fprintln(out, task.Title)
	meta := []string{"Status: " + statusWords(task.Status), "Kind: " + kindWords(task.Kind)}
	if name := appName(task); name != "" {
		meta = append(meta, "App: "+name)
	}
	meta = append(meta, "Opened: "+formatTimestamp(task.CreatedAt))
	if task.FinishedAt != nil && *task.FinishedAt != "" {
		meta = append(meta, "Finished: "+formatTimestamp(*task.FinishedAt))
	}
	fmt.Fprintln(out, strings.Join(meta, " · "))
	if brief := strings.TrimSpace(task.Brief); brief != "" {
		fmt.Fprintf(out, "Why: %s\n", brief)
	}

	pendingByStep := map[string]client.Approval{}
	var pendingWhole []client.Approval
	for _, approval := range detail.Approvals {
		if approval.Status != "pending" {
			continue
		}
		if approval.StepID == nil || *approval.StepID == "" {
			pendingWhole = append(pendingWhole, approval)
			continue
		}
		pendingByStep[*approval.StepID] = approval
	}

	if len(detail.Plan) > 0 {
		fmt.Fprintln(out, "\nPlan")
		for _, step := range detail.Plan {
			line := fmt.Sprintf("  %d. [%s] %s", step.Position, statusWords(step.Status), step.Title)
			var facts []string
			if step.Risk != "" {
				facts = append(facts, step.Risk+" risk")
			}
			if step.NeedsApproval || step.Risk != "" {
				if step.Reversible {
					facts = append(facts, "reversible")
				} else {
					facts = append(facts, "not reversible")
				}
			}
			if len(facts) > 0 {
				line += " (" + strings.Join(facts, ", ") + ")"
			}
			fmt.Fprintln(out, line)
			if detailText := strings.TrimSpace(step.Detail); detailText != "" {
				fmt.Fprintf(out, "     %s\n", detailText)
			}
			if step.Error != nil && strings.TrimSpace(*step.Error) != "" {
				fmt.Fprintf(out, "     Failed: %s\n", strings.TrimSpace(*step.Error))
			}
			if approval, waiting := pendingByStep[step.ID]; waiting {
				fmt.Fprintf(out, "     Needs your OK: %s\n     → tael approve %s   or   tael decline %s\n",
					approval.Summary, task.ID, task.ID)
			}
		}
	}
	for _, approval := range pendingWhole {
		fmt.Fprintf(out, "\nNeeds your OK: %s\n→ tael approve %s   or   tael decline %s\n", approval.Summary, task.ID, task.ID)
	}
	if task.Status == "proposed" && len(pendingWhole) == 0 {
		fmt.Fprintf(out, "\nTael proposes this.\n→ tael approve %s   or   tael decline %s\n", task.ID, task.ID)
	}

	if len(detail.Changes) > 0 {
		fmt.Fprintln(out, "\nChange")
		for _, change := range detail.Changes {
			line := "  " + strings.ReplaceAll(change.Subkind, "_", " ") + ": " + change.Title
			if change.URL != nil && *change.URL != "" {
				line += "  " + *change.URL
			}
			fmt.Fprintln(out, line)
		}
	}

	if len(detail.Evidence) > 0 {
		fmt.Fprintln(out, "\nEvidence")
		for _, item := range detail.Evidence {
			if item.Subkind == "transcript" {
				fmt.Fprintln(out, "  What I looked at:")
				for _, line := range transcriptLines(item.Body) {
					fmt.Fprintf(out, "    - %s\n", line)
				}
				continue
			}
			line := fmt.Sprintf("  %s %s", okMark(item.OK), item.Title)
			if item.URL != nil && *item.URL != "" {
				line += "  " + *item.URL
			}
			fmt.Fprintln(out, line)
			if body := strings.TrimSpace(item.Body); body != "" && item.Subkind != "link" {
				fmt.Fprintln(out, indentBlock(body, "      ", 12))
			}
		}
	}

	outcome := detail.Outcome
	if outcome == nil {
		outcome = task.Outcome
	}
	if outcome != nil && (strings.TrimSpace(outcome.Summary) != "" || strings.TrimSpace(outcome.Next) != "") {
		fmt.Fprintln(out, "\nOutcome")
		if summary := strings.TrimSpace(outcome.Summary); summary != "" {
			fmt.Fprintln(out, indentBlock(summary, "  ", 0))
		}
		if next := strings.TrimSpace(outcome.Next); next != "" {
			fmt.Fprintf(out, "  Next: %s\n", next)
		}
	}

	if len(detail.Comments) > 0 {
		fmt.Fprintln(out, "\nComments")
		for _, comment := range detail.Comments {
			author := "Tael"
			switch comment.Author {
			case "you":
				author = "You"
			case "member":
				author = "A teammate"
				if comment.AuthorName != nil && *comment.AuthorName != "" {
					author = *comment.AuthorName
				}
			}
			fmt.Fprintf(out, "  %s (%s): %s\n", author, formatTimestamp(comment.CreatedAt), strings.TrimSpace(comment.Body))
		}
	}
}
