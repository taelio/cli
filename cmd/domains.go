package cmd

import (
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"tael.io/cli/internal/client"
)

var domainsCmd = &cobra.Command{
	Use:   "domains",
	Short: "Every app's web address, with the live ones marked",
	Args:  cobra.NoArgs,
	RunE: func(command *cobra.Command, _ []string) error {
		listResponse, listError := apiClient.ListApps(command.Context())
		if listError != nil {
			return listError
		}
		if rendered, renderError := renderJSON(command, listResponse); rendered || renderError != nil {
			return renderError
		}
		if len(listResponse.Apps) == 0 {
			fmt.Fprintln(command.OutOrStdout(), "No apps yet, so no addresses. Run `tael new --repo owner/name` to connect a repository.")
			return nil
		}
		fmt.Fprint(command.OutOrStdout(), renderDomainsTable(listResponse.Apps))
		return nil
	},
}

func init() {
	rootCmd.AddCommand(domainsCmd)
}

// renderDomainsTable lists each app's address; live ones are marked.
func renderDomainsTable(apps []client.App) string {
	var builder strings.Builder
	table := tabwriter.NewWriter(&builder, 0, 0, 2, ' ', 0)
	fmt.Fprintln(table, "APP\tADDRESS\tLIVE")
	for _, app := range apps {
		live := "-"
		if app.Status == "live" && app.LiveURL != "" {
			live = "● live"
		}
		fmt.Fprintf(table, "%s\t%s\t%s\n", app.Name, valueOrDash(app.LiveURL), live)
	}
	_ = table.Flush()
	return builder.String()
}
