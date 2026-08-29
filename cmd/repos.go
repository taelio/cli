package cmd

import (
	"fmt"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"tael.io/cli/internal/client"
)

var reposCmd = &cobra.Command{
	Use:   "repos",
	Short: "List the repositories Tael can see, ready for `tael new`",
	Args:  cobra.NoArgs,
	RunE: func(command *cobra.Command, _ []string) error {
		listResponse, listError := apiClient.ListRepositories(command.Context())
		if listError != nil {
			return listError
		}
		if rendered, renderError := renderJSON(command, listResponse); rendered || renderError != nil {
			return renderError
		}
		out := command.OutOrStdout()
		if len(listResponse.Repos) == 0 {
			fmt.Fprintln(out, "Tael cannot see any repositories yet.")
			fmt.Fprintln(out, "Connecting GitHub is a browser step: open the web app, choose New app, and install the Tael GitHub App on the repositories you want.")
			return nil
		}
		fmt.Fprint(out, renderReposTable(sortRepos(listResponse.Repos)))
		fmt.Fprintln(out, "Put one live with `tael new --repo owner/name`.")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(reposCmd)
}

// sortRepos orders the way the picker does: repositories made with an app
// generator first, then most recently pushed first.
func sortRepos(repos []client.Repository) []client.Repository {
	sorted := append([]client.Repository(nil), repos...)
	sort.SliceStable(sorted, func(left int, right int) bool {
		leftGenerated := sorted[left].DetectedGenerator == "lovable"
		rightGenerated := sorted[right].DetectedGenerator == "lovable"
		if leftGenerated != rightGenerated {
			return leftGenerated
		}
		leftPushed, leftError := time.Parse(time.RFC3339, sorted[left].PushedAt)
		rightPushed, rightError := time.Parse(time.RFC3339, sorted[right].PushedAt)
		if leftError != nil || rightError != nil {
			return sorted[left].FullName < sorted[right].FullName
		}
		return leftPushed.After(rightPushed)
	})
	return sorted
}

// renderReposTable renders the repositories as an aligned table.
func renderReposTable(repos []client.Repository) string {
	var builder strings.Builder
	table := tabwriter.NewWriter(&builder, 0, 0, 2, ' ', 0)
	fmt.Fprintln(table, "REPOSITORY\tBRANCH\tLANGUAGE\tPUSHED\tNOTE")
	for _, repo := range repos {
		var notes []string
		if repo.Private {
			notes = append(notes, "private")
		}
		if repo.DetectedGenerator != "" {
			notes = append(notes, "made with "+repo.DetectedGenerator)
		}
		fmt.Fprintf(table, "%s\t%s\t%s\t%s\t%s\n",
			repo.FullName,
			valueOrDash(repo.DefaultBranch),
			valueOrDash(repo.Language),
			valueOrDash(formatTimestamp(repo.PushedAt)),
			valueOrDash(strings.Join(notes, ", ")),
		)
	}
	_ = table.Flush()
	return builder.String()
}
