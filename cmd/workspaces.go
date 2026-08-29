package cmd

import (
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"tael.io/cli/internal/client"
)

// tael workspaces — every workspace the person is in, and which one the
// CLI acts in. A token is made inside one workspace; `workspace use`
// names another, and says plainly when the token cannot follow.

var workspacesCmd = &cobra.Command{
	Use:   "workspaces",
	Short: "Every workspace you are in, and the one the CLI acts in",
	Args:  cobra.NoArgs,
	RunE: func(command *cobra.Command, _ []string) error {
		listResponse, listError := apiClient.ListWorkspaces(command.Context())
		if listError != nil {
			return listError
		}
		if rendered, renderError := renderJSON(command, listResponse); rendered || renderError != nil {
			return renderError
		}
		out := command.OutOrStdout()
		if len(listResponse.Workspaces) == 0 {
			fmt.Fprintln(out, "You are not in a workspace yet.")
			return nil
		}
		fmt.Fprint(out, renderWorkspacesTable(listResponse))
		fmt.Fprintln(out, "Act in another with `tael workspace use <slug>`.")
		return nil
	},
}

var workspaceCmd = &cobra.Command{
	Use:   "workspace",
	Short: "Choose the workspace the CLI acts in",
}

var workspaceUseCmd = &cobra.Command{
	Use:   "use <slug>",
	Short: "Act in this workspace from now on",
	Long: `Choose the workspace the CLI acts in. A token is made inside one
workspace; choosing another only takes effect where the API lets that
token follow, and the CLI says which it is.`,
	Args: cobra.ExactArgs(1),
	RunE: func(command *cobra.Command, args []string) error {
		out := command.OutOrStdout()
		listResponse, listError := apiClient.ListWorkspaces(command.Context())
		if listError != nil {
			return listError
		}
		chosen := findWorkspace(listResponse.Workspaces, args[0])
		if chosen == nil {
			slugs := make([]string, 0, len(listResponse.Workspaces))
			for _, workspace := range listResponse.Workspaces {
				slugs = append(slugs, workspace.Slug)
			}
			return withExitCode(exitUsage, fmt.Errorf("you are not in a workspace called %q; yours: %s", args[0], strings.Join(slugs, ", ")))
		}
		current := findWorkspaceByID(listResponse.Workspaces, listResponse.CurrentID)
		saved := readConfigFile()
		if chosen.ID == listResponse.CurrentID {
			saved.Workspace, saved.WorkspaceID = chosen.Slug, ""
			if writeError := writeConfigFile(saved); writeError != nil {
				return writeError
			}
			if rendered, renderError := renderJSON(command, map[string]any{"workspace": chosen, "acting": true}); rendered || renderError != nil {
				return renderError
			}
			fmt.Fprintf(out, "Acting in %s (%s): the workspace your token was made in.\n", chosen.Name, chosen.Slug)
			return nil
		}
		// Ask the API, with the header, which workspace it puts this token
		// in. Only a choice that took is kept.
		apiClient.WorkspaceID = chosen.ID
		whoami, whoamiError := apiClient.Whoami(command.Context())
		if whoamiError != nil {
			return whoamiError
		}
		if whoami.Workspace.ID != chosen.ID {
			currentName := "another workspace"
			if current != nil {
				currentName = fmt.Sprintf("%s (%s)", current.Name, current.Slug)
			}
			return fmt.Errorf("your token was made in %s and cannot act in %s: select %s in the web app and run `tael login` again, or make a token there with `tael tokens create`",
				currentName, chosen.Slug, chosen.Name)
		}
		saved.Workspace, saved.WorkspaceID = chosen.Slug, chosen.ID
		if writeError := writeConfigFile(saved); writeError != nil {
			return writeError
		}
		if rendered, renderError := renderJSON(command, map[string]any{"workspace": chosen, "acting": true}); rendered || renderError != nil {
			return renderError
		}
		fmt.Fprintf(out, "Now acting in %s (%s).\n", chosen.Name, chosen.Slug)
		return nil
	},
}

func init() {
	workspaceCmd.AddCommand(workspaceUseCmd)
	rootCmd.AddCommand(workspacesCmd, workspaceCmd)
}

func findWorkspace(workspaces []client.Membership, word string) *client.Membership {
	trimmed := strings.TrimSpace(word)
	for index := range workspaces {
		if workspaces[index].ID == trimmed || strings.EqualFold(workspaces[index].Slug, trimmed) || strings.EqualFold(workspaces[index].Name, trimmed) {
			return &workspaces[index]
		}
	}
	return nil
}

func findWorkspaceByID(workspaces []client.Membership, id string) *client.Membership {
	for index := range workspaces {
		if workspaces[index].ID == id {
			return &workspaces[index]
		}
	}
	return nil
}

// renderWorkspacesTable lists the workspaces with the one in use marked.
func renderWorkspacesTable(listResponse *client.ListWorkspacesResponse) string {
	var builder strings.Builder
	table := tabwriter.NewWriter(&builder, 0, 0, 2, ' ', 0)
	fmt.Fprintln(table, "SLUG\tNAME\tPLAN\tROLE\tACTING")
	for _, workspace := range listResponse.Workspaces {
		acting := "-"
		if workspace.ID == listResponse.CurrentID {
			acting = "● here"
		}
		fmt.Fprintf(table, "%s\t%s\t%s\t%s\t%s\n", workspace.Slug, workspace.Name, valueOrDash(workspace.Plan), valueOrDash(workspace.Role), acting)
	}
	_ = table.Flush()
	return builder.String()
}
