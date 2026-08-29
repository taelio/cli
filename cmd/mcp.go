package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// tael mcp — how to point an MCP client at this workspace. The endpoint
// speaks the Model Context Protocol over stateless Streamable HTTP and
// authenticates with the same tokens as the rest of the API, so the
// wiring is one URL and one header.

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Connect an MCP client (Claude Code, Cursor, ...) to your workspace",
	Args:  cobra.NoArgs,
	RunE: func(command *cobra.Command, _ []string) error {
		endpoint := resolveConfiguration(command).BaseURL + "/api/v1/mcp"
		if rendered, renderError := renderJSON(command, map[string]string{
			"endpoint":  endpoint,
			"transport": "http",
			"header":    "Authorization: Bearer <token>",
		}); rendered || renderError != nil {
			return renderError
		}
		out := command.OutOrStdout()
		fmt.Fprintf(out, "Tael speaks the Model Context Protocol at:\n\n  %s\n\n", endpoint)
		fmt.Fprint(out, "Authenticate with a workspace token — make one with `tael tokens create <name>` — then:\n\n")
		fmt.Fprintf(out, "  claude mcp add tael --transport http %s \\\n    --header \"Authorization: Bearer <token>\"\n\n", endpoint)
		fmt.Fprint(out, `Any other MCP client takes the same URL and header (transport "http" or
"streamable-http"). Read tools: list_apps, get_app, get_app_status, list_deploys,
get_app_logs, get_pipeline, get_workspace_status, list_incidents, list_tasks and
get_task. Write tools: deploy_app and create_task.
`)
		return nil
	},
}

func init() {
	setRequiresAuth(mcpCmd, false)
	rootCmd.AddCommand(mcpCmd)
}
