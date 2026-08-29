package cmd

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"tael.io/cli/internal/client"
)

var pipelineSetFlags []string

var pipelineCmd = &cobra.Command{
	Use:   "pipeline [app] [--set step.setting=value]",
	Short: "Show an app's pipeline, or change a step's setting",
	Long: `Show the steps that take a push to a live app, and what triggers them.

Change a step's setting with --set <step>.<setting>=<value>, for example
--set verify.path=/health. The triggers are settable too:
--set triggers.push_branches=main,develop and --set triggers.pull_request=true.
Tael opens a pull request with the change; nothing changes until it is merged.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(command *cobra.Command, args []string) error {
		appName, resolveError := resolveAppArgument(command, args)
		if resolveError != nil {
			return resolveError
		}
		pipeline, getError := apiClient.GetPipeline(command.Context(), appName)
		if getError != nil {
			return getError
		}
		if len(pipelineSetFlags) == 0 {
			if rendered, renderError := renderJSON(command, pipeline); rendered || renderError != nil {
				return renderError
			}
			fmt.Fprint(command.OutOrStdout(), renderPipeline(appName, pipeline))
			return nil
		}
		graph := pipeline.Graph
		for _, assignment := range pipelineSetFlags {
			if applyError := applyPipelineSetting(&graph, assignment); applyError != nil {
				return withExitCode(exitUsage, applyError)
			}
		}
		response, putError := apiClient.PutPipeline(command.Context(), appName, graph, pipeline.GraphVersion)
		if putError != nil {
			return putError
		}
		if rendered, renderError := renderJSON(command, response); rendered || renderError != nil {
			return renderError
		}
		note := response.Note
		if note == "" {
			note = "Tael is opening a pull request with this change."
		}
		fmt.Fprintf(command.OutOrStdout(), "Changed the pipeline for %s (now version %d). %s\n", appName, response.GraphVersion, note)
		return nil
	},
}

func init() {
	pipelineCmd.Flags().StringArrayVar(&pipelineSetFlags, "set", nil, "change a setting: <step>.<setting>=<value> (repeatable)")
	rootCmd.AddCommand(pipelineCmd)
}

// applyPipelineSetting changes one setting on the graph in place.
func applyPipelineSetting(graph *client.PipelineGraph, assignment string) error {
	target, value, hasValue := strings.Cut(assignment, "=")
	stepID, setting, hasSetting := strings.Cut(strings.TrimSpace(target), ".")
	if !hasValue || !hasSetting || stepID == "" || setting == "" {
		return fmt.Errorf("--set takes <step>.<setting>=<value>, got %q", assignment)
	}
	if stepID == "triggers" {
		switch setting {
		case "push_branches":
			branches := []string{}
			for _, branch := range strings.Split(value, ",") {
				if trimmed := strings.TrimSpace(branch); trimmed != "" {
					branches = append(branches, trimmed)
				}
			}
			graph.Triggers.PushBranches = branches
			return nil
		case "pull_request":
			enabled, parseError := strconv.ParseBool(strings.TrimSpace(value))
			if parseError != nil {
				return fmt.Errorf("triggers.pull_request takes true or false, got %q", value)
			}
			graph.Triggers.PullRequest = enabled
			return nil
		}
		return fmt.Errorf("triggers has push_branches and pull_request, not %q", setting)
	}
	for index := range graph.Nodes {
		if graph.Nodes[index].ID != stepID {
			continue
		}
		if graph.Nodes[index].With == nil {
			graph.Nodes[index].With = map[string]string{}
		}
		graph.Nodes[index].With[setting] = value
		return nil
	}
	names := make([]string, 0, len(graph.Nodes))
	for _, node := range graph.Nodes {
		names = append(names, node.ID)
	}
	return fmt.Errorf("no step called %q; steps: %s", stepID, strings.Join(names, ", "))
}

// renderPipeline prints the pipeline as the flowchart reads: what starts
// it, then each step with its settings and what it follows.
func renderPipeline(appName string, pipeline *client.Pipeline) string {
	var builder strings.Builder
	fmt.Fprintf(&builder, "Pipeline for %s (version %d)\n", appName, pipeline.GraphVersion)
	var triggers []string
	if len(pipeline.Graph.Triggers.PushBranches) > 0 {
		triggers = append(triggers, "a push to "+strings.Join(pipeline.Graph.Triggers.PushBranches, ", "))
	}
	if pipeline.Graph.Triggers.PullRequest {
		triggers = append(triggers, "a pull request")
	}
	if len(triggers) == 0 {
		triggers = append(triggers, "nothing yet")
	}
	fmt.Fprintf(&builder, "Runs on: %s\n", strings.Join(triggers, " and "))
	after := map[string][]string{}
	for _, edge := range pipeline.Graph.Edges {
		after[edge.To] = append(after[edge.To], edge.From)
	}
	table := tabwriter.NewWriter(&builder, 0, 0, 2, ' ', 0)
	fmt.Fprintln(table, "STEP\tDOES\tSETTINGS\tAFTER")
	for _, node := range pipeline.Graph.Nodes {
		keys := make([]string, 0, len(node.With))
		for key := range node.With {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		settings := make([]string, 0, len(keys))
		for _, key := range keys {
			settings = append(settings, key+"="+node.With[key])
		}
		name := node.Name
		if name == "" {
			name = node.Kind
		}
		fmt.Fprintf(table, "%s\t%s\t%s\t%s\n", node.ID, valueOrDash(name),
			valueOrDash(strings.Join(settings, " ")), valueOrDash(strings.Join(after[node.ID], ", ")))
	}
	_ = table.Flush()
	builder.WriteString("Change a setting with --set <step>.<setting>=<value>.\n")
	return builder.String()
}
