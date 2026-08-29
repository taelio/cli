package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"strings"

	"github.com/spf13/cobra"

	"tael.io/cli/internal/client"
)

// tael ask — a question to Tael AI, the answer streamed as it is written.
// The agent's frames name its tools for machines; what is printed is what
// Tael looked at, in its own words, never an identifier.

var askAppFlag string

var askCmd = &cobra.Command{
	Use:   "ask <question>",
	Short: "Ask Tael a question; the answer streams as it is written",
	Long: `Ask Tael anything about your apps: "why is web slow?", "what changed
this week?". Name an app with --app so the question is asked in front of
it. Tael says what it looks at along the way; interrupt with Ctrl+C.`,
	Args: cobra.ExactArgs(1),
	RunE: func(command *cobra.Command, args []string) error {
		question := strings.TrimSpace(args[0])
		if question == "" {
			return withExitCode(exitUsage, fmt.Errorf("ask Tael something"))
		}
		request := client.ChatRequest{Message: question, Context: client.ChatContext{Surface: "tael-cli"}}
		if askAppFlag != "" {
			appID, resolveError := resolveAppID(command.Context(), askAppFlag)
			if resolveError != nil {
				return resolveError
			}
			request.Context.AppID = appID
		}
		format, _ := parseOutputFormat(outputFlag)
		printer := newAnswerPrinter(command.OutOrStdout(), format == outputJSON)
		var answerError error
		askError := apiClient.Ask(command.Context(), request, func(frame client.SSEFrame) bool {
			classified := classifyChatFrame(frame)
			if classified.Kind == "error" {
				answerError = fmt.Errorf("%s", classified.Message)
				return false
			}
			printer.print(classified)
			return classified.Kind != "end"
		})
		printer.finish()
		if askError != nil {
			return askError
		}
		return answerError
	},
}

func init() {
	askCmd.Flags().StringVar(&askAppFlag, "app", "", "the app to ask in front of (name or id)")
	rootCmd.AddCommand(askCmd)
}

// chatFrame is a frame, classified: what to do with it.
type chatFrame struct {
	Kind    string `json:"kind"`
	Text    string `json:"text,omitempty"`
	Label   string `json:"label,omitempty"`
	OK      *bool  `json:"ok,omitempty"`
	ID      string `json:"id,omitempty"`
	Title   string `json:"title,omitempty"`
	Summary string `json:"summary,omitempty"`
	Risk    string `json:"risk,omitempty"`
	Message string `json:"message,omitempty"`
}

// answerPrinter writes the answer as it streams: text as it comes, what
// Tael looked at on its own lines, and a proposal or question set apart.
type answerPrinter struct {
	out       io.Writer
	asJSON    bool
	midLine   bool
	wroteText bool
}

func newAnswerPrinter(out io.Writer, asJSON bool) *answerPrinter {
	return &answerPrinter{out: out, asJSON: asJSON}
}

func (printer *answerPrinter) breakLine() {
	if printer.midLine {
		fmt.Fprintln(printer.out)
		printer.midLine = false
	}
}

func (printer *answerPrinter) print(frame chatFrame) {
	if printer.asJSON {
		if frame.Kind == "ignore" {
			return
		}
		encoded, _ := json.Marshal(frame)
		fmt.Fprintln(printer.out, string(encoded))
		return
	}
	switch frame.Kind {
	case "text":
		fmt.Fprint(printer.out, frame.Text)
		printer.midLine = !strings.HasSuffix(frame.Text, "\n")
		printer.wroteText = true
	case "tool_start":
		printer.breakLine()
		fmt.Fprintf(printer.out, "  · %s\n", frame.Label)
	case "tool_result":
		if frame.OK != nil && !*frame.OK {
			printer.breakLine()
			fmt.Fprintf(printer.out, "  · %s — that did not answer\n", frame.Label)
		}
	case "proposal":
		printer.breakLine()
		line := "Tael proposes: " + frame.Title
		if frame.Summary != "" {
			line += " — " + frame.Summary
		}
		if frame.Risk != "" {
			line += " (" + frame.Risk + " risk)"
		}
		fmt.Fprintln(printer.out, line)
		fmt.Fprintln(printer.out, "Decide it with `tael needs-you` once it is on the record.")
	case "question":
		printer.breakLine()
		fmt.Fprintf(printer.out, "Tael asks: %s\n", frame.Text)
	}
}

func (printer *answerPrinter) finish() {
	if printer.asJSON {
		return
	}
	printer.breakLine()
	if !printer.wroteText {
		fmt.Fprintln(printer.out, "Tael had nothing to add.")
	}
}

var (
	textFrameEvents  = map[string]bool{"content": true, "content_delta": true, "message": true, "delta": true, "text": true}
	toolStartEvents  = map[string]bool{"tool_start": true, "tool_start_live": true}
	endFrameEvents   = map[string]bool{"complete": true, "end": true, "turn_completed": true, "done": true}
	identifierMarker = regexp.MustCompile(`^[a-z0-9]+(?:[_-][a-z0-9]+)+$`)
)

// classifyChatFrame reads a frame the way the web app's drawer does: text
// to show, a tool Tael used, a proposal, a question, the end, an error.
func classifyChatFrame(frame client.SSEFrame) chatFrame {
	switch {
	case textFrameEvents[frame.Event]:
		text := textFromFrame(frame)
		if text == "" {
			return chatFrame{Kind: "ignore"}
		}
		return chatFrame{Kind: "text", Text: text}
	case toolStartEvents[frame.Event]:
		return chatFrame{Kind: "tool_start", Label: toolLabel(toolNameOf(payloadRecord(frame)))}
	case frame.Event == "tool_result":
		record := payloadRecord(frame)
		return chatFrame{Kind: "tool_result", Label: toolLabel(toolNameOf(record)), OK: readOK(record)}
	case frame.Event == "action_proposal":
		record := payloadRecord(frame)
		title := readString(record, "title", "action", "summary", "description")
		summary := readString(record, "summary", "description", "detail", "reason")
		if title == "" {
			title = "Tael proposes a change"
		}
		if summary == title {
			summary = ""
		}
		return chatFrame{Kind: "proposal", ID: readString(record, "action_id", "id", "proposal_id"), Title: title, Summary: summary,
			Risk: strings.ToLower(readString(record, "risk", "risk_level"))}
	case frame.Event == "ask_question":
		text := readString(payloadRecord(frame), "question", "text", "message", "content")
		if text == "" {
			return chatFrame{Kind: "ignore"}
		}
		return chatFrame{Kind: "question", Text: text}
	case endFrameEvents[frame.Event]:
		return chatFrame{Kind: "end"}
	case frame.Event == "error":
		message := readString(payloadRecord(frame), "detail", "error", "message")
		if message == "" {
			message = textFromFrame(frame)
		}
		if message == "" {
			message = "Tael ran into a problem answering that."
		}
		return chatFrame{Kind: "error", Message: message}
	}
	return chatFrame{Kind: "ignore"}
}

// textFromFrame pulls displayable text out of a frame's payload, reading
// the fields a delta plausibly carries rather than insisting on one.
func textFromFrame(frame client.SSEFrame) string {
	if frame.Data == "" {
		return ""
	}
	var payload any
	if unmarshalError := json.Unmarshal([]byte(frame.Data), &payload); unmarshalError != nil {
		return frame.Data // bare text, taken as written
	}
	switch value := payload.(type) {
	case string:
		return value
	case map[string]any:
		for _, field := range []string{"text", "delta", "content", "message", "value"} {
			switch candidate := value[field].(type) {
			case string:
				return candidate
			case map[string]any:
				if nested, isString := candidate["text"].(string); isString {
					return nested
				}
			}
		}
	}
	return ""
}

func payloadRecord(frame client.SSEFrame) map[string]any {
	if frame.Data == "" {
		return nil
	}
	var record map[string]any
	if unmarshalError := json.Unmarshal([]byte(frame.Data), &record); unmarshalError != nil {
		return nil
	}
	return record
}

func readString(record map[string]any, keys ...string) string {
	for _, key := range keys {
		switch value := record[key].(type) {
		case string:
			if strings.TrimSpace(value) != "" {
				return value
			}
		case map[string]any:
			if nested, isString := value["name"].(string); isString && strings.TrimSpace(nested) != "" {
				return nested
			}
		}
	}
	return ""
}

func readOK(record map[string]any) *bool {
	if record == nil {
		return nil
	}
	if ok, isBool := record["ok"].(bool); isBool {
		return &ok
	}
	if success, isBool := record["success"].(bool); isBool {
		return &success
	}
	if isError, isBool := record["is_error"].(bool); isBool {
		ok := !isError
		return &ok
	}
	if failure, isString := record["error"].(string); isString && failure != "" {
		return boolPointer(false)
	}
	if status, isString := record["status"].(string); isString && (status == "error" || status == "failed") {
		return boolPointer(false)
	}
	return nil
}

func boolPointer(value bool) *bool { return &value }

func toolNameOf(record map[string]any) string {
	return readString(record, "tool", "tool_name", "name", "function", "tool_call")
}

// toolLabels is what a tool call reads as, in Tael's voice. The agent's
// tools are named for machines; the person watching wants to know what
// Tael looked at. Nothing here names what is underneath.
var toolLabels = map[string]string{
	"get_pods":               "Looked at the pods",
	"list_pods":              "Looked at the pods",
	"describe_pod":           "Looked closely at a pod",
	"get_pod_logs":           "Read the logs",
	"pod_logs":               "Read the logs",
	"get_logs":               "Read the logs",
	"logs":                   "Read the logs",
	"get_events":             "Read the recent events",
	"list_events":            "Read the recent events",
	"get_deployments":        "Looked at the deployments",
	"get_deployment":         "Looked at the deployment",
	"describe_deployment":    "Looked closely at the deployment",
	"get_services":           "Looked at the services",
	"get_ingress":            "Looked at the routes in",
	"get_ingresses":          "Looked at the routes in",
	"get_nodes":              "Checked the machines",
	"get_node_metrics":       "Checked the machines' load",
	"get_metrics":            "Read the metrics",
	"get_pod_metrics":        "Checked what the pods use",
	"get_application":        "Looked at the app",
	"get_applications":       "Looked at your apps",
	"list_applications":      "Looked at your apps",
	"get_application_status": "Checked the app's status",
	"get_deploy":             "Looked at the deploy",
	"get_deploys":            "Looked at the deploys",
	"get_workflow_run":       "Looked at the build",
	"get_workflow_runs":      "Looked at the builds",
	"get_helm_release":       "Looked at the installed release",
	"list_helm_releases":     "Looked at what is installed",
	"get_stack":              "Looked at what is installed",
	"get_stacks":             "Looked at what is installed",
	"get_addon":              "Looked at the add-on",
	"get_cluster":            "Looked at your infrastructure",
	"get_cluster_status":     "Checked your infrastructure",
	"get_alerts":             "Read the alerts",
	"get_incidents":          "Read the incidents",
	"http_probe":             "Checked the app answers",
	"probe":                  "Checked the app answers",
	"probe_url":              "Checked the app answers",
	"run_kubectl":            "Ran a check on your infrastructure",
	"kubectl":                "Ran a check on your infrastructure",
	"exec_command":           "Ran a command in the app",
	"read_file":              "Read a file",
	"search_docs":            "Looked something up",
	"web_search":             "Looked something up",
	"think":                  "Thought it over",
}

var (
	toolVerbPrefixes      = []string{"get_", "list_", "describe_", "read_", "fetch_", "check_", "inspect_", "show_"}
	toolNamespacePrefixes = []string{"k8s_", "kubernetes_", "ankra_", "platform_", "tael_", "mcp_"}
	platformNames         = regexp.MustCompile(`(?i)ankra|kubernetes|k8s|helm|cluster|stack`)
)

func humanizeToolNoun(identifier string) string {
	spaced := strings.NewReplacer("_", " ", "-", " ").Replace(identifier)
	var words []string
	for _, word := range strings.Fields(platformNames.ReplaceAllString(spaced, "infrastructure")) {
		if len(words) > 0 && words[len(words)-1] == word {
			continue // "ankra stack" would otherwise read "infrastructure infrastructure"
		}
		words = append(words, word)
	}
	return strings.Join(words, " ")
}

// toolLabel is the line for a tool call: a known tool has its own words,
// an unknown one gets a plain sentence built from its noun, and a name
// with no readable noun reads as "Checked something". Whatever comes
// back is never an identifier and never names what is underneath.
func toolLabel(name string) string {
	key := strings.ToLower(strings.TrimSpace(name))
	if key == "" {
		return "Checked something"
	}
	if label, known := toolLabels[key]; known {
		return label
	}
	for _, prefix := range toolNamespacePrefixes {
		if strings.HasPrefix(key, prefix) {
			key = strings.TrimPrefix(key, prefix)
			break
		}
	}
	if label, known := toolLabels[key]; known {
		return label
	}
	if strings.Contains(key, "log") {
		return "Read the logs"
	}
	for _, prefix := range toolVerbPrefixes {
		if strings.HasPrefix(key, prefix) {
			noun := humanizeToolNoun(strings.TrimPrefix(key, prefix))
			if noun == "" {
				return "Checked something"
			}
			return "Looked at the " + noun
		}
	}
	noun := humanizeToolNoun(key)
	if noun == "" || identifierMarker.MatchString(noun) || regexp.MustCompile(`^[a-z0-9]+$`).MatchString(noun) {
		return "Checked something"
	}
	return "Checked the " + noun
}
