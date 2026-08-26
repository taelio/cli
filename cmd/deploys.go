package cmd

import (
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"tael.io/cli/internal/client"
)

var deploysCmd = &cobra.Command{
	Use:   "deploys [app]",
	Short: "List an app's deploy history",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(command *cobra.Command, args []string) error {
		appName, resolveError := resolveAppArgument(command, args)
		if resolveError != nil {
			return resolveError
		}
		listResponse, listError := apiClient.ListDeploys(command.Context(), appName)
		if listError != nil {
			return listError
		}
		if rendered, renderError := renderJSON(command, listResponse); rendered || renderError != nil {
			return renderError
		}
		if len(listResponse.Deploys) == 0 {
			fmt.Fprintf(command.OutOrStdout(), "No deploys yet for %s.\n", appName)
			return nil
		}
		fmt.Fprint(command.OutOrStdout(), renderDeploysTable(listResponse.Deploys))
		return nil
	},
}

func init() {
	rootCmd.AddCommand(deploysCmd)
}

// renderDeploysTable renders the deploy history as an aligned text table.
func renderDeploysTable(deploys []client.Deploy) string {
	var builder strings.Builder
	table := tabwriter.NewWriter(&builder, 0, 0, 2, ' ', 0)
	fmt.Fprintln(table, "ID\tSTATUS\tCOMMIT\tMESSAGE\tCREATED\tFINISHED")
	for _, deploy := range deploys {
		fmt.Fprintf(table, "%s\t%s\t%s\t%s\t%s\t%s\n",
			deploy.ID,
			valueOrDash(deploy.Status),
			valueOrDash(shortCommit(deploy.CommitSHA)),
			valueOrDash(firstLine(deploy.CommitMessage)),
			valueOrDash(formatTimestamp(deploy.CreatedAt)),
			valueOrDash(formatTimestamp(deploy.FinishedAt)),
		)
	}
	_ = table.Flush()
	return builder.String()
}

func shortCommit(commitSHA string) string {
	if len(commitSHA) > 7 {
		return commitSHA[:7]
	}
	return commitSHA
}

func firstLine(message string) string {
	line, _, _ := strings.Cut(message, "\n")
	if len(line) > 60 {
		return line[:57] + "..."
	}
	return line
}
