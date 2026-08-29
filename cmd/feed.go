package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"tael.io/cli/internal/client"
)

// tael feed — what Tael is doing, as the drawer narrates it: one line per
// thing that happened, in Tael's voice, never a step's internal name.

var (
	feedFollowFlag bool
	feedSinceFlag  int64
	feedLastFlag   int
)

// feedPageSize is how many events one replay read returns at most; the
// feed pages through to the newest. feedMaxPages bounds that on a
// workspace with a long history.
const (
	feedPageSize = 200
	feedMaxPages = 25
)

var feedCmd = &cobra.Command{
	Use:   "feed [--follow] [--since <event id>] [--last N]",
	Short: "What Tael is doing, in its own words; --follow keeps listening",
	Long: `Print the workspace's activity feed: the newest lines of what Tael did,
each with its time. With -f/--follow the CLI stays attached and prints new
lines as they happen; interrupt with Ctrl+C. --since resumes after an event
id (printed with -o json), --last says how many recent lines to show first.`,
	Args: cobra.NoArgs,
	RunE: func(command *cobra.Command, _ []string) error {
		out := command.OutOrStdout()
		format, _ := parseOutputFormat(outputFlag)
		asJSON := format == outputJSON

		recent, lastID, readError := readRecentEvents(command, feedSinceFlag, feedLastFlag, !asJSON)
		if readError != nil {
			return readError
		}
		if !feedFollowFlag && asJSON {
			return renderJSONAlways(command, client.ListEventsResponse{Events: recent})
		}
		if len(recent) == 0 && !feedFollowFlag {
			fmt.Fprintln(out, "Quiet. Lines appear here as Tael works.")
			return nil
		}
		for _, event := range recent {
			printFeedEvent(out, event, asJSON)
		}
		if !feedFollowFlag {
			return nil
		}
		return apiClient.FollowEventsSince(command.Context(), lastID, func(event client.Event) bool {
			printFeedEvent(out, event, asJSON)
			return true
		})
	},
}

func init() {
	feedCmd.Flags().BoolVarP(&feedFollowFlag, "follow", "f", false, "keep listening and print new lines as they happen")
	feedCmd.Flags().Int64Var(&feedSinceFlag, "since", 0, "start after this event id")
	feedCmd.Flags().IntVar(&feedLastFlag, "last", 30, "how many recent lines to show")
	rootCmd.AddCommand(feedCmd)
}

// renderJSONAlways writes value as JSON regardless of the flag; the feed
// decided already.
func renderJSONAlways(command *cobra.Command, value any) error {
	encoder := json.NewEncoder(command.OutOrStdout())
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}

// readRecentEvents pages through the stream from since to the newest and
// keeps the last `keep` events — only the ones with a line to show when
// narrated — returning them with the newest id seen, to resume from.
func readRecentEvents(command *cobra.Command, since int64, keep int, narrated bool) ([]client.Event, int64, error) {
	cursor := since
	var kept []client.Event
	for page := 0; page < feedMaxPages; page++ {
		listResponse, listError := apiClient.ListEvents(command.Context(), cursor)
		if listError != nil {
			return nil, cursor, listError
		}
		for _, event := range listResponse.Events {
			if narrated {
				if line, _ := narrateFeedEvent(event); line == "" {
					continue
				}
			}
			kept = append(kept, event)
		}
		if keep > 0 && len(kept) > keep {
			kept = kept[len(kept)-keep:]
		}
		if len(listResponse.Events) > 0 {
			cursor = listResponse.Events[len(listResponse.Events)-1].ID
		}
		if len(listResponse.Events) < feedPageSize {
			break
		}
	}
	return kept, cursor, nil
}

func printFeedEvent(out io.Writer, event client.Event, asJSON bool) {
	if asJSON {
		encoded, _ := json.Marshal(event)
		fmt.Fprintln(out, string(encoded))
		return
	}
	line, failure := narrateFeedEvent(event)
	if line == "" {
		return
	}
	mark := " "
	if failure {
		mark = "✗"
	}
	fmt.Fprintf(out, "%s %s %s\n", formatTimestamp(event.CreatedAt), mark, line)
}

// stepNarrationPhases is what each step the queue runs reads as, while it
// runs and once it is done. Keyed by the step's name, which is never
// printed; an unmapped step reads as something plain.
type stepPhases struct {
	started   string
	completed string
}

var feedStepNarration = map[string]stepPhases{
	"workspace_provision":                   {"Setting up your workspace.", "Your workspace is ready."},
	"workspace_create_ankra_organisation":   {"Reserving your own private space to run in.", "Your private space is reserved."},
	"workspace_mint_organisation_token":     {"Securing the connection to your infrastructure.", "Secure connection established."},
	"workspace_create_runtime":              {"Preparing somewhere for your apps to run.", "Your runtime is up."},
	"workspace_configure_domain":            {"Setting up web addresses for your apps.", "Web addresses are ready."},
	"environment_upgrade":                   {"Moving you to a dedicated runtime.", "You are on a dedicated runtime."},
	"environment_ensure_hetzner_credential": {"Preparing the dedicated runtime's account.", "The dedicated runtime's account is ready."},
	"environment_create_cluster":            {"Building your dedicated runtime.", "Your dedicated runtime is built."},
	"environment_watch_cluster":             {"Waiting for the dedicated runtime to come up.", "The dedicated runtime is up."},
	"environment_switch_runtime":            {"Moving your apps over.", "Your apps are on the dedicated runtime."},
	"app_onboard":                           {"Getting your app ready to go live.", "Your app is set up."},
	"app_create_application":                {"Registering your app.", "Your app is registered."},
	"app_watch_generation":                  {"Reading your repo and working out what it needs.", "I know what your app needs."},
	"app_watch_installation":                {"Rolling your app out.", "Your app is running."},
	"app_deploy_application":                {"Deploying your app.", "Deploy finished."},
	"app_deploy":                            {"Deploying your app.", "Deploy finished."},
	"app_handle_github_webhook":             {"A push came in; checking what it means.", "The push is dealt with."},
	"pipeline_update":                       {"Writing the pipeline change.", "The pipeline change is written."},
	"pipeline_compile_pull_request":         {"Opening a pull request with the pipeline change.", "The pipeline pull request is open."},
	"solution_install":                      {"Adding it to your infrastructure.", "It is installed and ready."},
	"solution_resolve_profile":              {"Looking up the newest version Tael publishes.", "Found the version to install."},
	"solution_preflight":                    {"Checking there is room for it.", "There is room for it."},
	"solution_instantiate":                  {"Installing it.", "Installed; waiting for it to come up."},
	"solution_watch_execution":              {"Watching it come up.", "It is up."},
	"solution_collect_connection":           {"Collecting the connection details.", "The connection details are ready."},
	"solution_bind_apps":                    {"Connecting it to your app.", "Connected to your app."},
	"solution_bind":                         {"Connecting it to your app.", "Connected to your app."},
	"solution_remove":                       {"Removing it.", "Removed; its volumes are released."},
	"solution_unbind_apps":                  {"Disconnecting it from your apps.", "Disconnected from your apps."},
	"solution_delete_stacks":                {"Taking it off your infrastructure.", "It is off your infrastructure."},
	"solution_watch_removal":                {"Waiting for it to go, then releasing its volumes.", "Gone, and its volumes are released."},
	"solution_upgrade":                      {"Applying the newer version.", "The newer version is running."},
	"solution_apply_update":                 {"Rolling the newer version out.", "The newer version is rolled out."},
	"solution_reconcile":                    {"Checking on your solutions.", "Your solutions are checked."},
	"task_investigate":                      {"Looking into it.", "I have looked into it."},
	"task_gather_evidence":                  {"Gathering the facts: the deploy, the pods, the logs.", "I have the facts."},
	"task_ask_tael":                         {"Thinking about what I found.", "I know what I would do."},
	"task_execute":                          {"Doing what you approved.", "Done what you approved."},
	"task_execute_step":                     {"Making the change you approved.", "The change is in."},
	"task_verify":                           {"Checking the change took.", "The change took."},
	"task_sync":                             {"Tidying up finished tasks.", "Finished tasks are tidied."},
	"app_probe":                             {"Checking your app answers.", "Your app answers."},
	"incident_from_alert":                   {"An alert came in — working out what it means.", "The alert is on the record as an incident."},
	"nightly_audit":                         {"Running the nightly look over your apps.", "The nightly look is done."},
	"security_findings":                     {"Checking your images for known weaknesses.", "The security check is done."},
}

// feedPayload is the part of any event's payload the feed reads.
type feedPayload struct {
	taskEventPayload
	Detail string `json:"detail"`
	Offer  string `json:"offer"`
	Error  string `json:"error"`
}

// payloadText is prose the payload carries for the person, or "". The
// step's name is deliberately not a candidate: it is an identifier.
func payloadText(payload feedPayload) string {
	for _, candidate := range []string{payload.Message, payload.Title, payload.Detail} {
		trimmed := strings.TrimSpace(candidate)
		if trimmed != "" && !identifierMarker.MatchString(trimmed) {
			return trimmed
		}
	}
	return ""
}

// narrateFeedEvent turns one event into a line of Tael's voice, and says
// whether it is a failure. An empty line means nothing to show.
func narrateFeedEvent(event client.Event) (line string, failure bool) {
	var payload feedPayload
	if len(event.Payload) > 0 {
		if unmarshalError := json.Unmarshal(event.Payload, &payload); unmarshalError != nil {
			return "", false
		}
	}
	if event.EventType == "tael_suggestion" {
		message := strings.TrimSpace(payload.Message)
		if message == "" {
			return "", false
		}
		if offer := strings.TrimSpace(payload.Offer); offer != "" {
			return message + " " + offer, false
		}
		return message, false
	}
	if taskLine, isTask, taskFailure := narrateTaskFeedEvent(event.EventType, payload.taskEventPayload); isTask {
		return taskLine, taskFailure
	}
	detail := payloadText(payload)
	phases, known := feedStepNarration[payload.Name]
	switch event.EventType {
	case "step_started":
		if detail != "" {
			return detail, false
		}
		if known {
			return phases.started, false
		}
		return "Working on it.", false
	case "step_completed":
		if detail != "" {
			return detail, false
		}
		if known {
			return phases.completed, false
		}
		return "That part is done.", false
	case "step_failed", "operation_failed":
		if detail != "" {
			return detail, true
		}
		return "Something didn't work — I'm looking into it.", true
	case "operation_created", "operation_completed":
		// The steps narrate the work; the envelope around them would only
		// say "Working on it." twice.
		return "", false
	case "app_analysis_progress":
		if detail != "" {
			return detail, false
		}
		return "Still reading the repo…", false
	}
	if detail != "" {
		return detail, false
	}
	if known {
		return phases.started, false
	}
	return "", false
}

// narrateTaskFeedEvent is the task events with their titles, the way the
// drawer reads them; isTask is false for anything else.
func narrateTaskFeedEvent(eventType string, payload taskEventPayload) (line string, isTask bool, failure bool) {
	switch eventType {
	case "task_created", "task_updated", "approval_requested", "approval_decided", "artifact_added":
	default:
		return "", false, false
	}
	title := strings.TrimSpace(payload.Title)
	if title == "" {
		title = "a task"
	}
	switch eventType {
	case "task_created":
		if payload.Status == "proposed" {
			return "I propose: " + title, true, false
		}
		return "Started: " + title, true, false
	case "task_updated":
		switch payload.Status {
		case "running":
			return "Working on: " + title, true, false
		case "needs_approval":
			return "Waiting for you: " + title, true, false
		case "done":
			return "Done: " + title, true, false
		case "failed":
			return "Couldn't finish: " + title, true, true
		case "declined":
			return "Declined: " + title, true, false
		case "paused":
			return "Paused: " + title, true, false
		case "superseded":
			return "Superseded: " + title, true, false
		}
		return "Updated: " + title, true, false
	case "approval_requested":
		if summary := strings.TrimSpace(payload.Summary); summary != "" {
			return "Needs your OK: " + summary, true, false
		}
		return "Needs your OK: " + title, true, false
	case "approval_decided":
		switch payload.Decision {
		case "approved":
			return "Approved — I'm on it: " + title, true, false
		case "auto":
			return "Pre-approved, so I went ahead: " + title, true, false
		case "declined":
			return "Declined: " + title, true, false
		}
		return "Decided: " + title, true, false
	case "artifact_added":
		what := strings.TrimSpace(payload.ArtifactTitle)
		if what == "" {
			what = title
		}
		if payload.Artifact == "change" {
			return "Changed something: " + what, true, false
		}
		if payload.OK != nil && !*payload.OK {
			return "Found a problem: " + what, true, true
		}
		return "Noted: " + what, true, false
	}
	return "", false, false
}
