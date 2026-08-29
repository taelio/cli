package cmd

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"tael.io/cli/internal/client"
)

// The architecture studio in the terminal. `tael architecture` draws the
// workspace as text: addresses, apps, solutions and the runtime, each with
// what it connects to. `tael plan "<sentence>"` asks Tael for the smallest
// set of changes that does what was asked and keeps it as the last plan;
// `tael build` carries that plan out, after asking. Nothing runs until the
// person says build.

var (
	architectureAppFlag   string
	architectureStackFlag string
)

var architectureCmd = &cobra.Command{
	Use:   "architecture [--app <app> | --stack <stack>]",
	Short: "The workspace as one picture: stacks, apps, solutions and the runtime",
	Long: `The workspace as one picture, in text: the addresses that reach your apps,
the stacks and the apps, the solutions they read, and the runtime they run
on, with what connects them. Suggestions are what Tael would change,
phrased as the sentence that asks for it. --app narrows the picture to one
app — its repository, addresses, solutions and the apps it calls — and
--stack to one stack's apps.`,
	Args: cobra.NoArgs,
	RunE: func(command *cobra.Command, _ []string) error {
		scope, scopeError := resolveScopeFlags(command, architectureAppFlag, architectureStackFlag)
		if scopeError != nil {
			return scopeError
		}
		graph, graphError := apiClient.GetArchitecture(command.Context(), scope)
		if graphError != nil {
			return graphError
		}
		if rendered, renderError := renderJSON(command, graph); rendered || renderError != nil {
			return renderError
		}
		fmt.Fprint(command.OutOrStdout(), renderArchitecture(graph, stackMembersFor(command.Context(), graph)))
		return nil
	},
}

var (
	planBuildFlag bool
	planYesFlag   bool
	planJSONFlag  bool
	planAppFlag   string
	planStackFlag string
)

var (
	buildPlanFlag  string
	buildYesFlag   bool
	buildAppFlag   string
	buildStackFlag string
)

var buildCmd = &cobra.Command{
	Use:   "build [--plan <file>] [--app <app> | --stack <stack>] [--yes]",
	Short: "Carry out the last plan: what `tael plan` proposed, after you confirm",
	Long: `Carry out the last plan that "tael plan" wrote, or the plan in the file given.
The changes are shown first and Tael asks before it starts; --yes answers
for you, which scripts need. Blocked changes are listed and skipped. Every
change that goes through is one Tael tells you about when it is ready;
anything refused is said in a sentence, and the command exits 1. --app and
--stack scope the build the way they scope the plan.`,
	Args: cobra.NoArgs,
	RunE: func(command *cobra.Command, _ []string) error {
		path, fromFlag := buildPlanFlag, buildPlanFlag != ""
		if !fromFlag {
			path = lastPlanPath()
		}
		plan, readError := readPlan(path, fromFlag)
		if readError != nil {
			return readError
		}
		scope, scopeError := resolveScopeFlags(command, buildAppFlag, buildStackFlag)
		if scopeError != nil {
			return scopeError
		}
		asJSON, formatError := wantsJSON(false)
		if formatError != nil {
			return formatError
		}
		out := command.OutOrStdout()
		if !asJSON {
			if summary := strings.TrimSpace(plan.Summary); summary != "" {
				fmt.Fprintln(out, summary)
			}
			fmt.Fprint(out, renderChangeRows(plan.Changes))
		}
		outcome, buildError := buildChanges(command, plan.Changes, buildYesFlag, asJSON, scope)
		if asJSON && outcome != nil {
			if encodeError := writeJSON(command, outcome); encodeError != nil {
				return encodeError
			}
		}
		if buildError == nil && outcome != nil && !fromFlag && len(outcome.Applied) > 0 {
			// The plan is done; a second `tael build` should not do it twice.
			_ = os.Remove(path)
		}
		return buildError
	},
}

func init() {
	architectureCmd.Flags().StringVar(&architectureAppFlag, "app", "", "Only this app (name or id) and what it touches")
	architectureCmd.Flags().StringVar(&architectureStackFlag, "stack", "", "Only this stack (name or id) and its apps")
	planCmd.Flags().BoolVar(&planBuildFlag, "build", false, "Carry the plan out straight away, after asking (with --yes, without asking)")
	planCmd.Flags().BoolVar(&planYesFlag, "yes", false, "With --build: do not ask before building")
	planCmd.Flags().BoolVar(&planJSONFlag, "json", false, "Print the plan as JSON (same as -o json)")
	planCmd.Flags().StringVar(&planAppFlag, "app", "", "Plan inside one app (name or id)")
	planCmd.Flags().StringVar(&planStackFlag, "stack", "", "Plan inside one stack (name or id)")
	buildCmd.Flags().StringVar(&buildPlanFlag, "plan", "", "A plan file to build instead of the last plan (as `tael plan -o json` prints it)")
	buildCmd.Flags().BoolVar(&buildYesFlag, "yes", false, "Do not ask before building")
	buildCmd.Flags().StringVar(&buildAppFlag, "app", "", "Build inside one app (name or id)")
	buildCmd.Flags().StringVar(&buildStackFlag, "stack", "", "Build inside one stack (name or id)")
	rootCmd.AddCommand(architectureCmd, buildCmd)
}

// --- the picture ---

// resolveScopeFlags turns --app or --stack into the scope the API takes:
// nothing for the whole workspace, one app, or one stack. Names resolve
// the way they do everywhere else, and an unknown one lists what there is.
func resolveScopeFlags(command *cobra.Command, appFlag string, stackFlag string) (string, error) {
	if appFlag != "" && stackFlag != "" {
		return "", withExitCode(exitUsage, fmt.Errorf("one scope at a time: --app or --stack, not both"))
	}
	switch {
	case appFlag != "":
		appID, resolveError := resolveAppID(command.Context(), appFlag)
		if resolveError != nil {
			return "", resolveError
		}
		return client.AppScope(appID), nil
	case stackFlag != "":
		stack, resolveError := resolveStackArgument(command.Context(), stackFlag)
		if resolveError != nil {
			return "", resolveError
		}
		return client.StackScope(stack.ID), nil
	}
	return "", nil
}

// stackMembersFor names the apps inside each stack card, keyed by the
// stack's node id. The graph folds members into their stack, so the stacks
// list supplies the names; when it cannot be read the picture stands
// without them.
func stackMembersFor(requestContext context.Context, graph *client.ArchitectureGraph) map[string][]client.StackApp {
	hasStacks := false
	for _, node := range graph.Nodes {
		if node.Kind == client.ArchitectureKindStack {
			hasStacks = true
			break
		}
	}
	if !hasStacks {
		return nil
	}
	listResponse, listError := apiClient.ListStacks(requestContext)
	if listError != nil {
		return nil
	}
	members := make(map[string][]client.StackApp, len(listResponse.Stacks))
	for _, stack := range listResponse.Stacks {
		members[client.ArchitectureKindStack+":"+stack.ID] = stack.Apps
	}
	return members
}

// renderArchitecture draws the picture in text, one section per kind, each
// node as its status, title and subtitle with its edges in words beneath.
// Stack members sit indented under their stack, runtime services under the
// runtime.
func renderArchitecture(graph *client.ArchitectureGraph, stackMembers map[string][]client.StackApp) string {
	titles := map[string]string{}
	byKind := map[string][]client.ArchitectureNode{}
	for _, node := range graph.Nodes {
		titles[node.ID] = node.Title
		byKind[node.Kind] = append(byKind[node.Kind], node)
	}
	outgoing := map[string][]client.ArchitectureEdge{}
	for _, edge := range graph.Edges {
		outgoing[edge.From] = append(outgoing[edge.From], edge)
	}

	var builder strings.Builder
	sections := []struct {
		heading string
		kind    string
	}{
		{"Repository", client.ArchitectureKindRepo},
		{"Addresses", client.ArchitectureKindDomain},
		{"Stacks", client.ArchitectureKindStack},
		{"Apps", client.ArchitectureKindApp},
		{"Solutions", client.ArchitectureKindSolution},
	}
	for _, section := range sections {
		nodes := byKind[section.kind]
		if len(nodes) == 0 {
			continue
		}
		builder.WriteString(section.heading + "\n")
		for _, node := range nodes {
			if section.kind == client.ArchitectureKindStack {
				writeStackNode(&builder, node, graph.Stacks, stackMembers[node.ID], titles, outgoing[node.ID])
				continue
			}
			writeArchitectureNode(&builder, node, "  ", titles, outgoing[node.ID])
		}
	}
	services := byKind[client.ArchitectureKindService]
	if runtimes := byKind[client.ArchitectureKindRuntime]; len(runtimes) > 0 || len(services) > 0 || graph.RuntimeServicesUnavailable {
		builder.WriteString("Runtime\n")
		for _, node := range runtimes {
			writeArchitectureNode(&builder, node, "  ", titles, outgoing[node.ID])
		}
		for _, node := range services {
			writeArchitectureNode(&builder, node, "    ", titles, outgoing[node.ID])
		}
		if graph.RuntimeServicesUnavailable {
			builder.WriteString("    The runtime did not answer; services will appear when it does.\n")
		}
	}
	known := map[string]bool{
		client.ArchitectureKindDomain: true, client.ArchitectureKindApp: true, client.ArchitectureKindSolution: true,
		client.ArchitectureKindRuntime: true, client.ArchitectureKindService: true,
		client.ArchitectureKindStack: true, client.ArchitectureKindRepo: true,
	}
	others := []client.ArchitectureNode{}
	for _, node := range graph.Nodes {
		if !known[node.Kind] {
			others = append(others, node)
		}
	}
	if len(others) > 0 {
		builder.WriteString("Other\n")
		for _, node := range others {
			writeArchitectureNode(&builder, node, "  ", titles, outgoing[node.ID])
		}
	}
	if len(graph.Suggestions) > 0 {
		builder.WriteString("Suggestions\n")
		for _, suggestion := range graph.Suggestions {
			line := "  - " + suggestion.Prompt
			if reason := strings.TrimSpace(suggestion.Reason); reason != "" {
				line += " — " + reason
			}
			builder.WriteString(line + "\n")
		}
		builder.WriteString("Ask for one with `tael plan \"<suggestion>\"`.\n")
	}
	return builder.String()
}

// writeStackNode writes a stack's composite row — its health, its name and
// how many apps it holds — with the member apps indented beneath, the way
// services sit under the runtime.
func writeStackNode(builder *strings.Builder, node client.ArchitectureNode, summaries []client.ArchitectureStackSummary, members []client.StackApp, titles map[string]string, edges []client.ArchitectureEdge) {
	if strings.TrimSpace(node.Subtitle) == "" {
		for _, summary := range summaries {
			if node.ID == client.ArchitectureKindStack+":"+summary.ID || node.ID == summary.ID {
				node.Subtitle = plural(summary.AppCount, "app", "apps")
				break
			}
		}
	}
	writeArchitectureNode(builder, node, "  ", titles, edges)
	for _, member := range members {
		if strings.TrimSpace(member.Status) == "" {
			builder.WriteString("    " + member.Name + "\n")
			continue
		}
		fmt.Fprintf(builder, "    %s %s  %s\n", toneDot(member.Tone), strings.ReplaceAll(member.Status, "_", " "), member.Name)
	}
}

// writeArchitectureNode writes one node line and its edges in words.
func writeArchitectureNode(builder *strings.Builder, node client.ArchitectureNode, indent string, titles map[string]string, edges []client.ArchitectureEdge) {
	status := strings.ReplaceAll(node.Status, "_", " ")
	if node.Proposed {
		status += ", proposed"
	}
	line := fmt.Sprintf("%s%s %s  %s", indent, toneDot(node.Tone), valueOrDash(status), node.Title)
	if subtitle := strings.TrimSpace(node.Subtitle); subtitle != "" {
		line += " — " + subtitle
	}
	builder.WriteString(line + "\n")
	for _, edge := range edges {
		builder.WriteString(indent + "    " + edgeWords(edge, titles) + "\n")
	}
}

// toneDot is the mark before the status word: the picture's colour, in
// one character.
func toneDot(tone string) string {
	switch tone {
	case "live":
		return "●"
	case "failed":
		return "✖"
	case "warning":
		return "▲"
	case "in_progress":
		return "◐"
	}
	return "○"
}

// edgeWords says an edge from the node's side: "routes to website-demo",
// "reads DATABASE_URL from Tael Managed Postgres", "runs on Dedicated
// machine", "requires Tael Managed Object Storage".
func edgeWords(edge client.ArchitectureEdge, titles map[string]string) string {
	target := titles[edge.To]
	if target == "" {
		target = edge.To
	}
	var words string
	switch edge.Kind {
	case client.ArchitectureEdgeRoutes:
		words = "routes to " + target
	case client.ArchitectureEdgeReads:
		if edge.Label != "" {
			words = "reads " + edge.Label + " from " + target
		} else {
			words = "reads from " + target
		}
	case client.ArchitectureEdgeRunsOn:
		words = "runs on " + target
	case client.ArchitectureEdgeRequires:
		words = "requires " + target
	case client.ArchitectureEdgeCalls:
		words = "calls " + target
		if edge.Label != "" {
			words += " (" + edge.Label + ")"
		}
	default:
		words = strings.ReplaceAll(edge.Kind, "_", " ") + " " + target
	}
	if edge.Proposed {
		words += " (proposed)"
	}
	return words
}

// --- the plan ---

// runArchitecturePlan is `tael plan "<sentence>"`: ask Tael, show the
// answer, keep it as the last plan, and with --build carry it out.
func runArchitecturePlan(command *cobra.Command, args []string) error {
	prompt := ""
	if len(args) > 0 {
		prompt = strings.TrimSpace(args[0])
	}
	if prompt == "" {
		return withExitCode(exitUsage, fmt.Errorf("say what you would like to change: tael plan \"<sentence>\""))
	}
	scope, scopeError := resolveScopeFlags(command, planAppFlag, planStackFlag)
	if scopeError != nil {
		return scopeError
	}
	plan, planError := apiClient.PlanArchitecture(command.Context(), prompt, scope)
	if planError != nil {
		return planError
	}
	if saveError := savePlan(lastPlanPath(), plan); saveError != nil {
		return saveError
	}
	asJSON, formatError := wantsJSON(planJSONFlag)
	if formatError != nil {
		return formatError
	}
	if !planBuildFlag {
		if asJSON {
			return writeJSON(command, plan)
		}
		fmt.Fprint(command.OutOrStdout(), renderArchitecturePlan(plan, true))
		return nil
	}
	if !asJSON {
		fmt.Fprint(command.OutOrStdout(), renderArchitecturePlan(plan, false))
	}
	outcome, buildError := buildChanges(command, plan.Changes, planYesFlag, asJSON, scope)
	if asJSON {
		if encodeError := writeJSON(command, map[string]any{"plan": plan, "build": outcome}); encodeError != nil {
			return encodeError
		}
	}
	if buildError == nil && outcome != nil && len(outcome.Applied) > 0 {
		_ = os.Remove(lastPlanPath())
	}
	return buildError
}

// renderArchitecturePlan prints the summary, the changes as numbered rows,
// the questions, and — unless the build follows at once — the reminder
// that nothing runs yet.
func renderArchitecturePlan(plan *client.ArchitecturePlan, reminder bool) string {
	var builder strings.Builder
	if summary := strings.TrimSpace(plan.Summary); summary != "" {
		builder.WriteString(summary + "\n")
	}
	if len(plan.Changes) > 0 {
		builder.WriteString("\n")
		builder.WriteString(renderChangeRows(plan.Changes))
	}
	if len(plan.Questions) > 0 {
		builder.WriteString("\nQuestions:\n")
		for _, question := range plan.Questions {
			builder.WriteString("  - " + strings.TrimSpace(question) + "\n")
		}
	}
	if !reminder {
		return builder.String()
	}
	builder.WriteString("\n")
	if len(plan.Changes) == 0 {
		builder.WriteString("Nothing to build.\n")
		return builder.String()
	}
	builder.WriteString("Nothing runs until you say `tael build`.\n")
	return builder.String()
}

// renderChangeRows numbers the changes: title, detail, and why one is
// blocked when it is.
func renderChangeRows(changes []client.ArchitectureChange) string {
	var builder strings.Builder
	if len(changes) == 0 {
		builder.WriteString("No changes.\n")
		return builder.String()
	}
	for index, change := range changes {
		fmt.Fprintf(&builder, "%3d. %s\n", index+1, change.Title)
		if detail := strings.TrimSpace(change.Detail); detail != "" {
			builder.WriteString("     " + detail + "\n")
		}
		if blocked := strings.TrimSpace(change.Blocked); blocked != "" {
			builder.WriteString("     Blocked: " + blocked + "\n")
		}
	}
	return builder.String()
}

// --- the build ---

// stdinIsTerminal says whether a person is at the keyboard, so `tael build`
// can ask. Tests replace it.
var stdinIsTerminal = func() bool {
	info, statError := os.Stdin.Stat()
	return statError == nil && info.Mode()&os.ModeCharDevice != 0
}

// buildChanges carries the changes out: blocked ones are skipped, the rest
// are confirmed (on a terminal, unless yes) and sent, within the scope when
// one is given. In text mode it prints one line per change applied and per
// refusal. The outcome comes back for a JSON caller, alongside the error a
// refusal exits with.
func buildChanges(command *cobra.Command, changes []client.ArchitectureChange, yes bool, asJSON bool, scope string) (*client.ArchitectureOutcome, error) {
	out := command.OutOrStdout()
	ready := []client.ArchitectureChange{}
	skipped := 0
	for _, change := range changes {
		if strings.TrimSpace(change.Blocked) != "" {
			skipped++
			continue
		}
		ready = append(ready, change)
	}
	if !asJSON && skipped > 0 {
		fmt.Fprintf(out, "Skipping %s (blocked).\n", plural(skipped, "change", "changes"))
	}
	if len(ready) == 0 {
		if !asJSON {
			fmt.Fprintln(out, "Nothing to build.")
		}
		return &client.ArchitectureOutcome{Applied: []client.ArchitectureApplied{}, Refused: []client.ArchitectureRefused{}}, nil
	}
	if !yes {
		if !stdinIsTerminal() {
			return nil, withExitCode(exitUsage, fmt.Errorf(
				"not a terminal, so Tael cannot ask; run again with --yes to build %s", plural(len(ready), "change", "changes")))
		}
		promptWriter := out
		if asJSON {
			promptWriter = command.ErrOrStderr()
		}
		confirmed, confirmError := confirm(command.InOrStdin(), promptWriter, fmt.Sprintf("Build %s? [y/N] ", plural(len(ready), "change", "changes")))
		if confirmError != nil {
			return nil, confirmError
		}
		if !confirmed {
			if !asJSON {
				fmt.Fprintln(out, "Nothing built.")
			}
			return &client.ArchitectureOutcome{Applied: []client.ArchitectureApplied{}, Refused: []client.ArchitectureRefused{}}, nil
		}
	}
	outcome, applyError := apiClient.ApplyArchitecture(command.Context(), ready, scope)
	if applyError != nil {
		return nil, applyError
	}
	if outcome.Applied == nil {
		outcome.Applied = []client.ArchitectureApplied{}
	}
	if outcome.Refused == nil {
		outcome.Refused = []client.ArchitectureRefused{}
	}
	if !asJSON {
		for _, applied := range outcome.Applied {
			fmt.Fprintf(out, "%s — Tael will tell you when it is ready.\n", progressive(applied.Change.Title))
		}
		for _, refused := range outcome.Refused {
			fmt.Fprintf(out, "Refused: %s — %s\n", refused.Change.Title, refused.Reason)
		}
		if remaining := len(ready) - len(outcome.Applied) - len(outcome.Refused); remaining > 0 {
			fmt.Fprintf(out, "%s not attempted; Tael stops at the first refusal.\n", plural(remaining, "change", "changes"))
		}
		if len(outcome.Applied) > 0 {
			fmt.Fprintln(out, "Follow them with `tael tasks`.")
		}
	}
	if len(outcome.Refused) > 0 {
		return outcome, withExitCode(exitError, fmt.Errorf("%s refused", plural(len(outcome.Refused), "change", "changes")))
	}
	return outcome, nil
}

// confirm asks the question and reads one line; only y or yes is a yes.
func confirm(in io.Reader, out io.Writer, question string) (bool, error) {
	fmt.Fprint(out, question)
	line, readError := bufio.NewReader(in).ReadString('\n')
	if readError != nil && !errors.Is(readError, io.EOF) {
		return false, fmt.Errorf("read the answer: %w", readError)
	}
	answer := strings.ToLower(strings.TrimSpace(line))
	return answer == "y" || answer == "yes", nil
}

// progressive turns a change's title into what is happening now: "Add X"
// becomes "Adding X", "Connect X to Y" becomes "Connecting X to Y".
func progressive(title string) string {
	verbs := []struct{ prefix, doing string }{
		{"Add ", "Adding "}, {"Connect ", "Connecting "}, {"Remove ", "Removing "}, {"Move ", "Moving "},
		{"Group ", "Grouping "}, {"Link ", "Linking "}, {"Create ", "Creating "},
	}
	for _, verb := range verbs {
		if strings.HasPrefix(title, verb.prefix) {
			return verb.doing + strings.TrimPrefix(title, verb.prefix)
		}
	}
	return title
}

// --- the plan file ---

const lastPlanFileName = "last-plan.json"

// lastPlanPath is where `tael plan` leaves the plan for `tael build`:
// beside the config file when TAEL_CONFIG names one, else
// ~/.tael/last-plan.json.
func lastPlanPath() string {
	if override := os.Getenv("TAEL_CONFIG"); override != "" {
		return filepath.Join(filepath.Dir(override), lastPlanFileName)
	}
	home, homeError := os.UserHomeDir()
	if homeError != nil {
		return filepath.Join(".tael", lastPlanFileName)
	}
	return filepath.Join(home, ".tael", lastPlanFileName)
}

// savePlan writes the plan as the API sent it, so the file is what
// `tael plan -o json` prints and what `tael build --plan` reads.
func savePlan(path string, plan *client.ArchitecturePlan) error {
	if directoryError := os.MkdirAll(filepath.Dir(path), 0o700); directoryError != nil {
		return fmt.Errorf("keep the plan: %w", directoryError)
	}
	encoded, marshalError := json.MarshalIndent(plan, "", "  ")
	if marshalError != nil {
		return fmt.Errorf("keep the plan: %w", marshalError)
	}
	if writeError := os.WriteFile(path, append(encoded, '\n'), 0o600); writeError != nil {
		return fmt.Errorf("keep the plan: %w", writeError)
	}
	return nil
}

// readPlan loads a plan file. Without one there is nothing to build, which
// is a usage error that says what to do first.
func readPlan(path string, fromFlag bool) (*client.ArchitecturePlan, error) {
	content, readError := os.ReadFile(path)
	switch {
	case errors.Is(readError, os.ErrNotExist) && fromFlag:
		return nil, withExitCode(exitUsage, fmt.Errorf("no plan at %s", path))
	case errors.Is(readError, os.ErrNotExist):
		return nil, withExitCode(exitUsage, fmt.Errorf("no plan yet: say `tael plan \"<what you want>\"` first"))
	case readError != nil:
		return nil, fmt.Errorf("read the plan: %w", readError)
	}
	var plan client.ArchitecturePlan
	if unmarshalError := json.Unmarshal(content, &plan); unmarshalError != nil {
		return nil, withExitCode(exitUsage, fmt.Errorf("%s is not a plan `tael plan` wrote: %w", path, unmarshalError))
	}
	return &plan, nil
}

// --- output ---

// wantsJSON says whether output goes out as JSON: -o json, or a command's
// own --json shorthand.
func wantsJSON(shorthand bool) (bool, error) {
	format, formatError := parseOutputFormat(outputFlag)
	if formatError != nil {
		return false, withExitCode(exitUsage, formatError)
	}
	return shorthand || format == outputJSON, nil
}

// writeJSON writes value as indented JSON.
func writeJSON(command *cobra.Command, value any) error {
	encoder := json.NewEncoder(command.OutOrStdout())
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}
