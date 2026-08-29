package cmd

import (
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"tael.io/cli/internal/client"
)

var suggestionsAllFlag bool

var suggestionsCmd = &cobra.Command{
	Use:   "suggestions",
	Short: "What Tael noticed without being asked; `suggestions resolve <id>` marks one dealt with",
	Args:  cobra.NoArgs,
	RunE: func(command *cobra.Command, _ []string) error {
		listResponse, listError := apiClient.ListSuggestions(command.Context(), suggestionsAllFlag)
		if listError != nil {
			return listError
		}
		if rendered, renderError := renderJSON(command, listResponse); rendered || renderError != nil {
			return renderError
		}
		out := command.OutOrStdout()
		if len(listResponse.Suggestions) == 0 {
			fmt.Fprintln(out, "Nothing to suggest right now. Tael says so here when it notices something.")
			return nil
		}
		appNames := map[string]string{}
		if apps, appsError := apiClient.ListApps(command.Context()); appsError == nil {
			for _, app := range apps.Apps {
				appNames[app.ID] = app.Name
			}
		}
		fmt.Fprint(out, renderSuggestionsTable(listResponse.Suggestions, appNames))
		return nil
	},
}

var suggestionsResolveCmd = &cobra.Command{
	Use:   "resolve <id>",
	Short: "Mark a suggestion as dealt with",
	Args:  cobra.ExactArgs(1),
	RunE: func(command *cobra.Command, args []string) error {
		if resolveError := apiClient.ResolveSuggestion(command.Context(), args[0]); resolveError != nil {
			return resolveError
		}
		if rendered, renderError := renderJSON(command, map[string]string{"id": args[0], "status": "resolved"}); rendered || renderError != nil {
			return renderError
		}
		fmt.Fprintln(command.OutOrStdout(), "Marked as dealt with.")
		return nil
	},
}

func init() {
	suggestionsCmd.Flags().BoolVar(&suggestionsAllFlag, "all", false, "include suggestions already dealt with")
	suggestionsCmd.AddCommand(suggestionsResolveCmd)
	rootCmd.AddCommand(suggestionsCmd)
}

// renderSuggestionsTable renders the suggestions as an aligned table; the
// app is named when the list of apps could be read.
func renderSuggestionsTable(suggestions []client.Suggestion, appNames map[string]string) string {
	var builder strings.Builder
	table := tabwriter.NewWriter(&builder, 0, 0, 2, ' ', 0)
	fmt.Fprintln(table, "ID\tAPP\tTAEL NOTICED\tOFFERS\tWHEN")
	for _, suggestion := range suggestions {
		app := ""
		if suggestion.AppID != nil {
			app = appNames[*suggestion.AppID]
			if app == "" {
				app = *suggestion.AppID
			}
		}
		when := formatTimestamp(suggestion.CreatedAt)
		if suggestion.ResolvedAt != nil && *suggestion.ResolvedAt != "" {
			when += " (dealt with)"
		}
		fmt.Fprintf(table, "%s\t%s\t%s\t%s\t%s\n",
			suggestion.ID,
			valueOrDash(app),
			valueOrDash(strings.TrimSpace(suggestion.Message)),
			valueOrDash(strings.TrimSpace(suggestion.Offer)),
			valueOrDash(when),
		)
	}
	_ = table.Flush()
	return builder.String()
}
