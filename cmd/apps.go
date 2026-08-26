package cmd

import (
	"fmt"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"tael.io/cli/internal/client"
)

var appsCmd = &cobra.Command{
	Use:     "apps",
	Aliases: []string{"ps"},
	Short:   "List the apps in your workspace",
	Args:    cobra.NoArgs,
	RunE: func(command *cobra.Command, _ []string) error {
		listResponse, listError := apiClient.ListApps(command.Context())
		if listError != nil {
			return listError
		}
		if rendered, renderError := renderJSON(command, listResponse); rendered || renderError != nil {
			return renderError
		}
		if len(listResponse.Apps) == 0 {
			fmt.Fprintln(command.OutOrStdout(), "No apps yet. Run `tael init` to connect a repository.")
			return nil
		}
		fmt.Fprint(command.OutOrStdout(), renderAppsTable(listResponse.Apps))
		return nil
	},
}

func init() {
	rootCmd.AddCommand(appsCmd)
}

// renderAppsTable renders the apps list as an aligned text table.
func renderAppsTable(apps []client.App) string {
	var builder strings.Builder
	table := tabwriter.NewWriter(&builder, 0, 0, 2, ' ', 0)
	fmt.Fprintln(table, "NAME\tSTATUS\tURL\tUPDATED")
	for _, app := range apps {
		fmt.Fprintf(table, "%s\t%s\t%s\t%s\n",
			app.Name,
			valueOrDash(app.Status),
			valueOrDash(app.LiveURL),
			valueOrDash(formatTimestamp(app.UpdatedAt)),
		)
	}
	_ = table.Flush()
	return builder.String()
}

// valueOrDash substitutes "-" for an empty cell so table columns stay
// visually aligned and scannable.
func valueOrDash(value string) string {
	if value == "" {
		return "-"
	}
	return value
}

// formatTimestamp renders an RFC3339 timestamp in the local timezone,
// falling back to the raw string for anything the platform sends that does
// not parse.
func formatTimestamp(raw string) string {
	parsed, parseError := time.Parse(time.RFC3339, raw)
	if parseError != nil {
		return raw
	}
	return parsed.Local().Format("2006-01-02 15:04")
}

// resolveAppArgument returns the app to operate on: the positional argument
// when given, otherwise the workspace's only app. With zero or several apps
// and no argument, the error lists what is available.
func resolveAppArgument(command *cobra.Command, args []string) (string, error) {
	if len(args) > 0 {
		return args[0], nil
	}
	listResponse, listError := apiClient.ListApps(command.Context())
	if listError != nil {
		return "", listError
	}
	switch len(listResponse.Apps) {
	case 0:
		return "", fmt.Errorf("no apps in this workspace: run `tael init` to connect a repository")
	case 1:
		return listResponse.Apps[0].Name, nil
	default:
		names := make([]string, 0, len(listResponse.Apps))
		for _, app := range listResponse.Apps {
			names = append(names, app.Name)
		}
		return "", withExitCode(exitUsage, fmt.Errorf(
			"several apps in this workspace; name one: %s", strings.Join(names, ", ")))
	}
}
