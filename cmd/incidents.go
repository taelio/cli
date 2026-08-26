package cmd

import (
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"tael.io/cli/internal/client"
)

var incidentsCmd = &cobra.Command{
	Use:   "incidents",
	Short: "List incidents in your workspace",
	Args:  cobra.NoArgs,
	RunE: func(command *cobra.Command, _ []string) error {
		listResponse, listError := apiClient.ListIncidents(command.Context())
		if listError != nil {
			return listError
		}
		if rendered, renderError := renderJSON(command, listResponse); rendered || renderError != nil {
			return renderError
		}
		if len(listResponse.Incidents) == 0 {
			fmt.Fprintln(command.OutOrStdout(), "No incidents.")
			return nil
		}
		fmt.Fprint(command.OutOrStdout(), renderIncidentsTable(listResponse.Incidents))
		return nil
	},
}

func init() {
	rootCmd.AddCommand(incidentsCmd)
}

// renderIncidentsTable renders the incidents list as an aligned text table.
func renderIncidentsTable(incidents []client.Incident) string {
	var builder strings.Builder
	table := tabwriter.NewWriter(&builder, 0, 0, 2, ' ', 0)
	fmt.Fprintln(table, "ID\tSEVERITY\tSTATUS\tTITLE\tCREATED")
	for _, incident := range incidents {
		fmt.Fprintf(table, "%s\t%s\t%s\t%s\t%s\n",
			incident.ID,
			valueOrDash(incident.Severity),
			valueOrDash(incident.Status),
			valueOrDash(incident.Title),
			valueOrDash(formatTimestamp(incident.CreatedAt)),
		)
	}
	_ = table.Flush()
	return builder.String()
}
