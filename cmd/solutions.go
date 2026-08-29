package cmd

import (
	"context"
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"tael.io/cli/internal/client"
)

// tael solutions — the Tael Managed solutions on the workspace's runtime:
// list what is installed, add one from the catalog, check on one, connect
// it to an app, remove it. A solution is named by its display name, its
// instance name or its id; add takes the catalog key (postgres, monitoring,
// object-storage, backups, secrets).

var solutionsCmd = &cobra.Command{
	Use:   "solutions",
	Short: "Manage Tael Managed solutions: a database, monitoring, storage, backups",
	Long: `Tael Managed solutions are installed on your infrastructure, watched, kept
up to date and connected to your apps. List what is installed, add one from
the catalog, check on one, connect it to an app, or remove it.`,
}

var solutionsListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List the solutions installed in your workspace",
	Args:    cobra.NoArgs,
	RunE: func(command *cobra.Command, _ []string) error {
		listResponse, listError := apiClient.ListSolutions(command.Context())
		if listError != nil {
			return listError
		}
		if rendered, renderError := renderJSON(command, listResponse); rendered || renderError != nil {
			return renderError
		}
		installed := presentSolutions(listResponse.Solutions)
		out := command.OutOrStdout()
		if len(installed) == 0 {
			fmt.Fprintln(out, "No solutions installed.")
			if catalog, catalogError := apiClient.GetSolutionCatalog(command.Context()); catalogError == nil {
				fmt.Fprint(out, renderCatalogHint(catalog.Solutions))
			}
			return nil
		}
		fmt.Fprint(out, renderSolutionsTable(installed))
		return nil
	},
}

var (
	solutionsAddForFlag  string
	solutionsAddSizeFlag string
)

var solutionsAddCmd = &cobra.Command{
	Use:   "add <key>",
	Short: "Add a solution from the catalog (postgres, monitoring, object-storage, backups, secrets)",
	Args:  cobra.ExactArgs(1),
	RunE: func(command *cobra.Command, args []string) error {
		request := client.InstallSolutionRequest{SolutionKey: args[0], Preset: solutionsAddSizeFlag}
		if solutionsAddForFlag != "" {
			appID, resolveError := resolveAppID(command.Context(), solutionsAddForFlag)
			if resolveError != nil {
				return resolveError
			}
			request.AppID = appID
		}
		installResponse, installError := apiClient.InstallSolution(command.Context(), request)
		if installError != nil {
			return installError
		}
		if rendered, renderError := renderJSON(command, installResponse); rendered || renderError != nil {
			return renderError
		}
		out := command.OutOrStdout()
		fmt.Fprintf(out, "Adding %s. Tael will say when it is ready.\n", args[0])
		if len(installResponse.Required) > 0 {
			fmt.Fprintf(out, "%d more added first because it needs them.\n", len(installResponse.Required))
		}
		fmt.Fprintf(out, "Follow it with `tael solutions status %s`.\n", installResponse.ID)
		return nil
	},
}

var solutionsRemoveForceFlag bool

var solutionsRemoveCmd = &cobra.Command{
	Use:   "remove <name>",
	Short: "Remove a solution; stored data is deleted and volumes are released",
	Args:  cobra.ExactArgs(1),
	RunE: func(command *cobra.Command, args []string) error {
		solution, resolveError := resolveSolutionArgument(command.Context(), args[0])
		if resolveError != nil {
			return resolveError
		}
		operation, removeError := apiClient.RemoveSolution(command.Context(), solution.ID, solutionsRemoveForceFlag)
		if removeError != nil {
			return removeError
		}
		if rendered, renderError := renderJSON(command, operation); rendered || renderError != nil {
			return renderError
		}
		fmt.Fprintf(command.OutOrStdout(), "Removing %s. Stored data is deleted; volumes are released.\n", solution.Name)
		return nil
	},
}

var solutionsStatusCmd = &cobra.Command{
	Use:   "status <name>",
	Short: "Show a solution's live status and checks",
	Args:  cobra.ExactArgs(1),
	RunE: func(command *cobra.Command, args []string) error {
		solution, resolveError := resolveSolutionArgument(command.Context(), args[0])
		if resolveError != nil {
			return resolveError
		}
		statusResponse, statusError := apiClient.GetSolutionStatus(command.Context(), solution.ID)
		if statusError != nil {
			return statusError
		}
		if rendered, renderError := renderJSON(command, statusResponse); rendered || renderError != nil {
			return renderError
		}
		fmt.Fprint(command.OutOrStdout(), renderSolutionStatus(solution, statusResponse))
		return nil
	},
}

var solutionsConnectAppFlag string

var solutionsConnectCmd = &cobra.Command{
	Use:   "connect <name> --app <app>",
	Short: "Connect a solution to an app; the app reads it as environment variables on its next deploy",
	Args:  cobra.ExactArgs(1),
	RunE: func(command *cobra.Command, args []string) error {
		solution, resolveError := resolveSolutionArgument(command.Context(), args[0])
		if resolveError != nil {
			return resolveError
		}
		appID, appError := resolveAppID(command.Context(), solutionsConnectAppFlag)
		if appError != nil {
			return appError
		}
		binding, bindError := apiClient.BindSolution(command.Context(), solution.ID, appID)
		if bindError != nil {
			return bindError
		}
		if rendered, renderError := renderJSON(command, binding); rendered || renderError != nil {
			return renderError
		}
		appName := binding.AppName
		if appName == "" {
			appName = solutionsConnectAppFlag
		}
		fmt.Fprintf(command.OutOrStdout(), "Connecting %s to %s. The values arrive on the app's next deploy.\n", solution.Name, appName)
		return nil
	},
}

func init() {
	solutionsAddCmd.Flags().StringVar(&solutionsAddForFlag, "for", "", "App the solution is for (a database is made for one app)")
	solutionsAddCmd.Flags().StringVar(&solutionsAddSizeFlag, "size", "", "Size: small, medium or large (the catalog's default when unset)")
	solutionsRemoveCmd.Flags().BoolVar(&solutionsRemoveForceFlag, "force", false, "Disconnect the apps that use it, then remove")
	solutionsConnectCmd.Flags().StringVar(&solutionsConnectAppFlag, "app", "", "App to connect it to (name or id)")
	_ = solutionsConnectCmd.MarkFlagRequired("app")

	solutionsCmd.AddCommand(solutionsListCmd, solutionsAddCmd, solutionsRemoveCmd, solutionsStatusCmd, solutionsConnectCmd)
	rootCmd.AddCommand(solutionsCmd)
}

// presentSolutions drops rows that are gone: removed, or retired with a
// runtime that was moved away from.
func presentSolutions(solutions []client.Solution) []client.Solution {
	present := make([]client.Solution, 0, len(solutions))
	for _, solution := range solutions {
		if solution.Status == "removed" || solution.Status == "retired" {
			continue
		}
		present = append(present, solution)
	}
	return present
}

// renderSolutionsTable renders the installed solutions as an aligned table.
func renderSolutionsTable(solutions []client.Solution) string {
	var builder strings.Builder
	table := tabwriter.NewWriter(&builder, 0, 0, 2, ' ', 0)
	fmt.Fprintln(table, "NAME\tSTATUS\tSIZE\tFOR\tCONNECTED\tINSTALLED")
	for _, solution := range solutions {
		appName := ""
		if solution.App != nil {
			appName = solution.App.Name
		}
		connected := make([]string, 0, len(solution.Bindings))
		for _, binding := range solution.Bindings {
			connected = append(connected, binding.AppName)
		}
		status := solution.Status
		if solution.UpdateAvailable {
			status += " (update available)"
		}
		fmt.Fprintf(table, "%s\t%s\t%s\t%s\t%s\t%s\n",
			solution.Name,
			valueOrDash(status),
			valueOrDash(solution.PresetLabel),
			valueOrDash(appName),
			valueOrDash(strings.Join(connected, ", ")),
			valueOrDash(formatTimestamp(solution.InstalledAt)),
		)
	}
	_ = table.Flush()
	return builder.String()
}

// renderCatalogHint lists what can be added, so an empty list is a start
// rather than a dead end.
func renderCatalogHint(entries []client.CatalogEntry) string {
	var builder strings.Builder
	builder.WriteString("Add one with `tael solutions add <key>`:\n")
	table := tabwriter.NewWriter(&builder, 0, 0, 2, ' ', 0)
	for _, entry := range entries {
		label := entry.Availability.Label
		if entry.Availability.State == "available" {
			label = ""
		}
		fmt.Fprintf(table, "  %s\t%s\t%s\n", entry.Key, entry.Name, label)
	}
	_ = table.Flush()
	return builder.String()
}

// renderSolutionStatus renders one solution's live status, the way
// `tael status` renders an app's.
func renderSolutionStatus(solution *client.Solution, status *client.SolutionStatusResponse) string {
	var builder strings.Builder
	fmt.Fprintf(&builder, "Solution: %s\n", solution.Name)
	if solution.PresetLabel != "" {
		fmt.Fprintf(&builder, "Size:     %s\n", solution.PresetLabel)
	}
	fmt.Fprintf(&builder, "Status:   %s\n", status.Status)
	fmt.Fprintf(&builder, "Healthy:  %t\n", status.Healthy)
	if solution.App != nil {
		fmt.Fprintf(&builder, "For:      %s\n", solution.App.Name)
	}
	if len(solution.Bindings) > 0 {
		names := make([]string, 0, len(solution.Bindings))
		for _, binding := range solution.Bindings {
			names = append(names, binding.AppName)
		}
		fmt.Fprintf(&builder, "Apps:     %s\n", strings.Join(names, ", "))
	}
	if len(status.Checks) > 0 {
		builder.WriteString("\nChecks:\n")
		for _, check := range status.Checks {
			line := fmt.Sprintf("  %-8s %s", check.Status, check.Name)
			if check.Message != "" {
				line += " — " + check.Message
			}
			builder.WriteString(line + "\n")
		}
	}
	if len(status.Pods) > 0 && !status.Healthy {
		builder.WriteString("\nPods:\n")
		for _, pod := range status.Pods {
			line := fmt.Sprintf("  %-10s %s", pod.Phase, pod.Name)
			if pod.Ready != "" {
				line += " " + pod.Ready + " ready"
			}
			if pod.Restarts > 0 {
				line += fmt.Sprintf(", %d restarts", pod.Restarts)
			}
			builder.WriteString(line + "\n")
		}
	}
	return builder.String()
}

// resolveSolutionArgument finds the installed solution the person named:
// by id, by instance name, or by display name. Rows that are gone are not
// candidates. An unknown name lists what there is.
func resolveSolutionArgument(requestContext context.Context, word string) (*client.Solution, error) {
	listResponse, listError := apiClient.ListSolutions(requestContext)
	if listError != nil {
		return nil, listError
	}
	installed := presentSolutions(listResponse.Solutions)
	for index := range installed {
		if client.MatchesSolution(installed[index], word) {
			return &installed[index], nil
		}
	}
	if len(installed) == 0 {
		return nil, withExitCode(exitUsage, fmt.Errorf("no solutions installed; add one with `tael solutions add <key>`"))
	}
	names := make([]string, 0, len(installed))
	for _, solution := range installed {
		names = append(names, solution.Name)
	}
	return nil, withExitCode(exitUsage, fmt.Errorf("no solution called %q; installed: %s", word, strings.Join(names, ", ")))
}

// resolveAppID turns an app's name or id into its id, since the solutions
// API takes ids while people say names.
func resolveAppID(requestContext context.Context, word string) (string, error) {
	trimmed := strings.TrimSpace(word)
	if trimmed == "" {
		return "", withExitCode(exitUsage, fmt.Errorf("say which app: --app <name>"))
	}
	listResponse, listError := apiClient.ListApps(requestContext)
	if listError != nil {
		return "", listError
	}
	for _, app := range listResponse.Apps {
		if app.ID == trimmed || strings.EqualFold(app.Name, trimmed) {
			return app.ID, nil
		}
	}
	names := make([]string, 0, len(listResponse.Apps))
	for _, app := range listResponse.Apps {
		names = append(names, app.Name)
	}
	if len(names) == 0 {
		return "", withExitCode(exitUsage, fmt.Errorf("no apps in this workspace: run `tael init` to connect a repository"))
	}
	return "", withExitCode(exitUsage, fmt.Errorf("no app called %q; apps: %s", trimmed, strings.Join(names, ", ")))
}
