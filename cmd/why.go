package cmd

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"tael.io/cli/internal/client"
)

var whyCmd = &cobra.Command{
	Use:   "why [app]",
	Short: "Why the last deploy of an app failed, in Tael's words",
	Long: `Print the outcome of the newest failed task for an app. When there is
none, ask Tael to look into why the last deploy failed and follow the
investigation until it finishes or needs you; interrupt with Ctrl+C.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(command *cobra.Command, args []string) error {
		app, resolveError := resolveAppArgument(command, args)
		if resolveError != nil {
			return resolveError
		}
		out := command.OutOrStdout()

		listResponse, listError := apiClient.ListTasks(command.Context(), "done", app)
		if listError != nil {
			return listError
		}
		if failed := newestFailedTask(listResponse.Tasks); failed != nil {
			if rendered, renderError := renderJSON(command, failed); rendered || renderError != nil {
				return renderError
			}
			fmt.Fprint(out, renderWhy(*failed))
			return nil
		}

		brief := fmt.Sprintf("Why did the last deploy of %s fail?", app)
		detail, createError := apiClient.CreateTask(command.Context(), brief, app, "investigate")
		if createError != nil {
			return createError
		}
		if rendered, renderError := renderJSON(command, detail); rendered || renderError != nil {
			return renderError
		}
		task := detail.Task
		if task.Status == "proposed" {
			fmt.Fprintf(out, "Asked Tael: %s\nTael can't run that on this workspace yet; the task is on the record: tael task %s\n",
				task.Title, task.ID)
			return nil
		}
		fmt.Fprintf(out, "Asked Tael: %s\nFollowing the investigation (Ctrl+C to stop; `tael task %s` later).\n\n", task.Title, task.ID)

		var finalStatus string
		followError := apiClient.FollowEvents(command.Context(), func(event client.Event) bool {
			line, status, ok := narrateTaskEvent(event, task.ID)
			if !ok {
				return true
			}
			if line != "" {
				fmt.Fprintln(out, line)
			}
			if isTerminalTaskStatus(status) {
				finalStatus = status
				return false
			}
			return true
		})
		if followError != nil {
			return followError
		}
		if finalStatus == "" {
			// The stream ended or the person stopped following; the task is
			// still running and its page has the rest.
			return nil
		}

		latest, getError := apiClient.GetTask(command.Context(), task.ID)
		if getError != nil {
			return getError
		}
		fmt.Fprintln(out)
		if finalStatus == "needs_approval" {
			fmt.Fprintf(out, "Tael needs your OK before it goes further: tael task %s\n", task.ID)
		}
		fmt.Fprint(out, renderWhy(latest.Task))
		return nil
	},
}

func init() {
	rootCmd.AddCommand(whyCmd)
}

// newestFailedTask picks the most recently finished failed task, or nil.
// The API lists newest first, so the first failed one is it.
func newestFailedTask(tasks []client.Task) *client.Task {
	for index := range tasks {
		if tasks[index].Status == "failed" {
			return &tasks[index]
		}
	}
	return nil
}

// renderWhy prints a task's outcome as an answer: what happened and what
// Tael would do next, with the task to open for the evidence.
func renderWhy(task client.Task) string {
	var builder strings.Builder
	fmt.Fprintf(&builder, "%s (%s, %s)\n", task.Title, statusWords(task.Status), taskWhen(task))
	if task.Outcome != nil && strings.TrimSpace(task.Outcome.Summary) != "" {
		fmt.Fprintln(&builder, strings.TrimSpace(task.Outcome.Summary))
	} else {
		fmt.Fprintln(&builder, "Tael has not written up what happened yet.")
	}
	if task.Outcome != nil && strings.TrimSpace(task.Outcome.Next) != "" {
		fmt.Fprintf(&builder, "Next: %s\n", strings.TrimSpace(task.Outcome.Next))
	}
	fmt.Fprintf(&builder, "Evidence: tael task %s\n", task.ID)
	return builder.String()
}

// isTerminalTaskStatus is true once following would show nothing more:
// the task finished one way or another, or it is waiting on a person.
func isTerminalTaskStatus(status string) bool {
	switch status {
	case "done", "failed", "declined", "superseded", "paused", "needs_approval":
		return true
	}
	return false
}

// taskEventPayload is the part of a task event's payload the CLI reads.
// Everything in it is prose or a Tael id; nothing names the platform.
type taskEventPayload struct {
	TaskID        string `json:"task_id"`
	Title         string `json:"title"`
	Status        string `json:"status"`
	Name          string `json:"name"`
	Summary       string `json:"summary"`
	Decision      string `json:"decision"`
	Artifact      string `json:"artifact"`
	ArtifactTitle string `json:"artifact_title"`
	OK            *bool  `json:"ok"`
	Message       string `json:"message"`
}

// stepNarration is what each step of an investigation reads as while it
// runs. Keyed by the queue's step name, which is never printed.
var stepNarration = map[string]string{
	"task_investigate":     "Looking into it.",
	"task_gather_evidence": "Gathering the facts: the deploy, the pods, the logs.",
	"task_ask_tael":        "Thinking about what I found.",
	"task_execute":         "Doing what you approved.",
	"task_execute_step":    "Making the change you approved.",
	"task_verify":          "Checking the change took.",
}

// narrateTaskEvent turns one stream event into a line for the person
// following a task. It answers ok=false for events about other things,
// and the task's status when the event carries one so the caller knows
// when to stop. Lines are Tael's words: never an identifier.
func narrateTaskEvent(event client.Event, taskID string) (line string, status string, ok bool) {
	var payload taskEventPayload
	if len(event.Payload) > 0 {
		if unmarshalError := json.Unmarshal(event.Payload, &payload); unmarshalError != nil {
			return "", "", false
		}
	}
	switch event.EventType {
	case "task_created", "task_updated", "approval_requested", "approval_decided", "artifact_added":
		if payload.TaskID != taskID {
			return "", "", false
		}
	case "step_started":
		if narration, known := stepNarration[payload.Name]; known {
			return narration, "", true
		}
		return "", "", false
	default:
		return "", "", false
	}
	switch event.EventType {
	case "task_created":
		return "Started.", payload.Status, true
	case "task_updated":
		switch payload.Status {
		case "running":
			return "Working on it.", payload.Status, true
		case "needs_approval":
			return "Needs your OK.", payload.Status, true
		case "done":
			return "Done.", payload.Status, true
		case "failed":
			return "Couldn't finish.", payload.Status, true
		case "declined":
			return "Declined.", payload.Status, true
		case "paused":
			return "Paused.", payload.Status, true
		case "superseded":
			return "Superseded by a newer task.", payload.Status, true
		}
		return "", payload.Status, true
	case "approval_requested":
		if payload.Summary != "" {
			return "Needs your OK: " + payload.Summary, "needs_approval", true
		}
		return "Needs your OK.", "needs_approval", true
	case "approval_decided":
		switch payload.Decision {
		case "approved":
			return "Approved — on it.", "", true
		case "auto":
			return "Pre-approved, so going ahead.", "", true
		case "declined":
			return "Declined.", "", true
		}
		return "Decided.", "", true
	case "artifact_added":
		what := payload.ArtifactTitle
		if what == "" {
			return "", "", true
		}
		if payload.Artifact == "change" {
			return "Changed something: " + what, "", true
		}
		if payload.OK != nil && !*payload.OK {
			return "Found a problem: " + what, "", true
		}
		return "Noted: " + what, "", true
	}
	return "", "", false
}
