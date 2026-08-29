package cmd

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"tael.io/cli/internal/client"
)

// tael tokens — the personal access tokens for this workspace. A token's
// secret is printed once, when it is made, and never again.

var tokensExpiresFlag string

var tokensCmd = &cobra.Command{
	Use:   "tokens",
	Short: "Your API tokens for this workspace; `tokens create <name>` makes one, `tokens revoke <id>` ends one",
	Args:  cobra.NoArgs,
	RunE: func(command *cobra.Command, _ []string) error {
		listResponse, listError := apiClient.ListTokens(command.Context())
		if listError != nil {
			return listError
		}
		if rendered, renderError := renderJSON(command, listResponse); rendered || renderError != nil {
			return renderError
		}
		out := command.OutOrStdout()
		if len(listResponse.Tokens) == 0 {
			fmt.Fprintln(out, "No tokens. Make one with `tael tokens create <name>`.")
			return nil
		}
		fmt.Fprint(out, renderTokensTable(listResponse.Tokens))
		return nil
	},
}

var tokensCreateCmd = &cobra.Command{
	Use:   "create <name> [--expires 30d|YYYY-MM-DD]",
	Short: "Make a token; its secret is shown this once",
	Args:  cobra.ExactArgs(1),
	RunE: func(command *cobra.Command, args []string) error {
		name := strings.TrimSpace(args[0])
		if name == "" {
			return withExitCode(exitUsage, fmt.Errorf("give the token a name, so you know it in the list"))
		}
		request := client.CreateTokenRequest{Name: name}
		if tokensExpiresFlag != "" {
			expiresAt, parseError := parseExpiry(tokensExpiresFlag, time.Now())
			if parseError != nil {
				return withExitCode(exitUsage, parseError)
			}
			request.ExpiresAt = &expiresAt
		}
		created, createError := apiClient.CreateToken(command.Context(), request)
		if createError != nil {
			return createError
		}
		if rendered, renderError := renderJSON(command, created); rendered || renderError != nil {
			return renderError
		}
		out := command.OutOrStdout()
		fmt.Fprintf(out, "Token made: %s\n\n  %s\n\n", created.Name, created.Token)
		fmt.Fprintf(out, "This is the only time it is shown. Use it with --token or %s; it expires %s.\n", envAPIToken, formatTimestamp(created.ExpiresAt))
		return nil
	},
}

var tokensRevokeCmd = &cobra.Command{
	Use:   "revoke <id or name>",
	Short: "Make a token stop working",
	Args:  cobra.ExactArgs(1),
	RunE: func(command *cobra.Command, args []string) error {
		token, resolveError := resolveTokenArgument(command.Context(), args[0])
		if resolveError != nil {
			return resolveError
		}
		if revokeError := apiClient.RevokeToken(command.Context(), token.ID); revokeError != nil {
			return revokeError
		}
		if rendered, renderError := renderJSON(command, map[string]string{"id": token.ID, "status": "revoked"}); rendered || renderError != nil {
			return renderError
		}
		fmt.Fprintf(command.OutOrStdout(), "Revoked %s. Anything using it stops working now.\n", token.Name)
		return nil
	},
}

func init() {
	tokensCreateCmd.Flags().StringVar(&tokensExpiresFlag, "expires", "", "when it stops working: a number of days (30d) or a date (2026-12-31); 90 days when unset")
	tokensCmd.AddCommand(tokensCreateCmd, tokensRevokeCmd)
	rootCmd.AddCommand(tokensCmd)
}

// parseExpiry reads "30d" or "YYYY-MM-DD" into an RFC 3339 instant.
func parseExpiry(word string, now time.Time) (string, error) {
	trimmed := strings.TrimSpace(word)
	if days, isDays := strings.CutSuffix(trimmed, "d"); isDays {
		count, parseError := strconv.Atoi(days)
		if parseError != nil || count <= 0 {
			return "", fmt.Errorf("--expires takes a number of days (30d) or a date (2026-12-31), got %q", word)
		}
		return now.Add(time.Duration(count) * 24 * time.Hour).UTC().Format(time.RFC3339), nil
	}
	date, parseError := time.ParseInLocation("2006-01-02", trimmed, time.Local)
	if parseError != nil {
		return "", fmt.Errorf("--expires takes a number of days (30d) or a date (2026-12-31), got %q", word)
	}
	if !date.After(now) {
		return "", fmt.Errorf("--expires must be in the future, got %q", word)
	}
	return date.UTC().Format(time.RFC3339), nil
}

// tokenState is one word for where a token stands.
func tokenState(token client.Token, now time.Time) string {
	if token.RevokedAt != nil && *token.RevokedAt != "" {
		return "revoked"
	}
	if token.ExpiresAt != nil {
		if expiresAt, parseError := time.Parse(time.RFC3339, *token.ExpiresAt); parseError == nil && expiresAt.Before(now) {
			return "expired"
		}
	}
	return "working"
}

// renderTokensTable lists the tokens; never their secrets.
func renderTokensTable(tokens []client.Token) string {
	var builder strings.Builder
	table := tabwriter.NewWriter(&builder, 0, 0, 2, ' ', 0)
	fmt.Fprintln(table, "NAME\tID\tSTATE\tMADE\tEXPIRES\tLAST USED")
	now := time.Now()
	for _, token := range tokens {
		fmt.Fprintf(table, "%s\t%s\t%s\t%s\t%s\t%s\n",
			valueOrDash(token.Name),
			token.ID,
			tokenState(token, now),
			valueOrDash(formatTimestamp(token.CreatedAt)),
			valueOrDash(formatTimestamp(stringOrEmpty(token.ExpiresAt))),
			valueOrDash(formatTimestamp(stringOrEmpty(token.LastUsedAt))),
		)
	}
	_ = table.Flush()
	return builder.String()
}

// resolveTokenArgument finds the token by id or name among the working
// ones; a name shared by several asks for the id.
func resolveTokenArgument(requestContext context.Context, word string) (*client.Token, error) {
	trimmed := strings.TrimSpace(word)
	if trimmed == "" {
		return nil, withExitCode(exitUsage, fmt.Errorf("say which token: its id or name"))
	}
	listResponse, listError := apiClient.ListTokens(requestContext)
	if listError != nil {
		return nil, listError
	}
	var byName []client.Token
	for _, token := range listResponse.Tokens {
		if token.ID == trimmed {
			return &token, nil
		}
		if strings.EqualFold(token.Name, trimmed) && tokenState(token, time.Now()) == "working" {
			byName = append(byName, token)
		}
	}
	switch len(byName) {
	case 1:
		return &byName[0], nil
	case 0:
		return nil, withExitCode(exitUsage, fmt.Errorf("no working token called %q; see `tael tokens`", trimmed))
	default:
		ids := make([]string, 0, len(byName))
		for _, token := range byName {
			ids = append(ids, token.ID)
		}
		return nil, withExitCode(exitUsage, fmt.Errorf("%d tokens are called %q; say which by id: %s", len(byName), trimmed, strings.Join(ids, ", ")))
	}
}
