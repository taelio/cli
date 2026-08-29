package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// tael team — the workspace's settings about its people: how it admits
// them.

var teamGithubOrgFlag string

var teamCmd = &cobra.Command{
	Use:   "team",
	Short: "Workspace settings about its people: how it admits them",
}

var teamJoinPolicyCmd = &cobra.Command{
	Use:   "join-policy [--github-org on|off]",
	Short: "Show or change how people join: by invitation only, or anyone with access to the GitHub repositories",
	Long: `Show how the workspace admits people. With --github-org on, anyone whose
GitHub account can see the workspace's repositories may join on their own;
with --github-org off it is by invitation only. Owners and admins only.`,
	Args: cobra.NoArgs,
	RunE: func(command *cobra.Command, _ []string) error {
		out := command.OutOrStdout()
		if teamGithubOrgFlag == "" {
			listResponse, listError := apiClient.ListMembers(command.Context())
			if listError != nil {
				return listError
			}
			if rendered, renderError := renderJSON(command, map[string]string{"join_policy": listResponse.JoinPolicy}); rendered || renderError != nil {
				return renderError
			}
			fmt.Fprintf(out, "Joining: %s.\nChange it with --github-org on|off.\n", joinPolicyWords(listResponse.JoinPolicy))
			return nil
		}
		var joinPolicy string
		switch teamGithubOrgFlag {
		case "on":
			joinPolicy = "github_repo_access"
		case "off":
			joinPolicy = "invite_only"
		default:
			return withExitCode(exitUsage, fmt.Errorf("--github-org takes on or off, not %q", teamGithubOrgFlag))
		}
		response, setError := apiClient.SetJoinPolicy(command.Context(), joinPolicy)
		if setError != nil {
			return setError
		}
		if rendered, renderError := renderJSON(command, response); rendered || renderError != nil {
			return renderError
		}
		fmt.Fprintf(out, "Joining is now %s.\n", joinPolicyWords(response.JoinPolicy))
		return nil
	},
}

func init() {
	teamJoinPolicyCmd.Flags().StringVar(&teamGithubOrgFlag, "github-org", "", "on: anyone with access to the repositories may join; off: by invitation only")
	teamCmd.AddCommand(teamJoinPolicyCmd)
	rootCmd.AddCommand(teamCmd)
}
