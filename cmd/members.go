package cmd

import (
	"context"
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"tael.io/cli/internal/client"
)

// tael members — who is in the workspace, changing a person's role, and
// taking someone out. A person is named by their GitHub login, email, name
// or id.

var membersCmd = &cobra.Command{
	Use:     "members",
	Aliases: []string{"member"},
	Short:   "Who is in the workspace; `member role` changes a role, `members remove` takes someone out",
	Args:    cobra.NoArgs,
	RunE: func(command *cobra.Command, _ []string) error {
		listResponse, listError := apiClient.ListMembers(command.Context())
		if listError != nil {
			return listError
		}
		if rendered, renderError := renderJSON(command, listResponse); rendered || renderError != nil {
			return renderError
		}
		fmt.Fprint(command.OutOrStdout(), renderMembers(listResponse))
		return nil
	},
}

var membersRemoveCmd = &cobra.Command{
	Use:   "remove <user>",
	Short: "Take a person out of the workspace (owners and admins only)",
	Args:  cobra.ExactArgs(1),
	RunE: func(command *cobra.Command, args []string) error {
		member, resolveError := resolveMemberArgument(command.Context(), args[0])
		if resolveError != nil {
			return resolveError
		}
		if removeError := apiClient.RemoveMember(command.Context(), member.UserID); removeError != nil {
			return removeError
		}
		if rendered, renderError := renderJSON(command, map[string]string{"user_id": member.UserID, "status": "removed"}); rendered || renderError != nil {
			return renderError
		}
		fmt.Fprintf(command.OutOrStdout(), "Removed %s from the workspace.\n", memberName(*member))
		return nil
	},
}

var membersRoleCmd = &cobra.Command{
	Use:   "role <user> <role>",
	Short: "Change a person's role: owner, admin or member (owners and admins only)",
	Args:  cobra.ExactArgs(2),
	RunE: func(command *cobra.Command, args []string) error {
		role := strings.ToLower(strings.TrimSpace(args[1]))
		if role != "owner" && role != "admin" && role != "member" {
			return withExitCode(exitUsage, fmt.Errorf("a role is owner, admin or member, not %q", args[1]))
		}
		member, resolveError := resolveMemberArgument(command.Context(), args[0])
		if resolveError != nil {
			return resolveError
		}
		if updateError := apiClient.UpdateMemberRole(command.Context(), member.UserID, role); updateError != nil {
			return updateError
		}
		if rendered, renderError := renderJSON(command, map[string]string{"user_id": member.UserID, "role": role, "status": "changed"}); rendered || renderError != nil {
			return renderError
		}
		fmt.Fprintf(command.OutOrStdout(), "%s is now %s.\n", memberName(*member), roleWithArticle(role))
		return nil
	},
}

func init() {
	membersCmd.AddCommand(membersRemoveCmd, membersRoleCmd)
	rootCmd.AddCommand(membersCmd)
}

// roleWithArticle says a role as a phrase: "an owner", "an admin", "a member".
func roleWithArticle(role string) string {
	switch role {
	case "owner", "admin":
		return "an " + role
	}
	return "a " + role
}

// memberName is what a person is called: their name, else their GitHub
// login, else their email.
func memberName(member client.Member) string {
	for _, candidate := range []*string{member.Name, member.GithubLogin, member.Email} {
		if candidate != nil && strings.TrimSpace(*candidate) != "" {
			return *candidate
		}
	}
	return member.UserID
}

func stringOrEmpty(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

// joinPolicyWords says how the workspace admits people.
func joinPolicyWords(joinPolicy string) string {
	switch joinPolicy {
	case "github_repo_access":
		return "anyone with access to the workspace's GitHub repositories can join"
	case "invite_only", "":
		return "by invitation only"
	}
	return strings.ReplaceAll(joinPolicy, "_", " ")
}

// renderMembers renders the people list, then how the workspace admits
// people and what the reader may do.
func renderMembers(listResponse *client.ListMembersResponse) string {
	var builder strings.Builder
	table := tabwriter.NewWriter(&builder, 0, 0, 2, ' ', 0)
	fmt.Fprintln(table, "NAME\tGITHUB\tEMAIL\tROLE\tJOINED")
	for _, member := range listResponse.Members {
		fmt.Fprintf(table, "%s\t%s\t%s\t%s\t%s\n",
			memberName(member),
			valueOrDash(stringOrEmpty(member.GithubLogin)),
			valueOrDash(stringOrEmpty(member.Email)),
			valueOrDash(member.Role),
			valueOrDash(formatTimestamp(member.JoinedAt)),
		)
	}
	_ = table.Flush()
	fmt.Fprintf(&builder, "Joining: %s.", joinPolicyWords(listResponse.JoinPolicy))
	switch listResponse.YourRole {
	case "owner", "admin":
		builder.WriteString(" Invite someone with `tael invite`.")
	case "":
	default:
		builder.WriteString(" Only an owner or admin can invite or remove people.")
	}
	builder.WriteString("\n")
	return builder.String()
}

// resolveMemberArgument finds the member the person named: by id, GitHub
// login, email or name (case-insensitively).
func resolveMemberArgument(requestContext context.Context, word string) (*client.Member, error) {
	trimmed := strings.TrimSpace(word)
	if trimmed == "" {
		return nil, withExitCode(exitUsage, fmt.Errorf("say who: a GitHub login, email, name or id"))
	}
	listResponse, listError := apiClient.ListMembers(requestContext)
	if listError != nil {
		return nil, listError
	}
	for index := range listResponse.Members {
		member := listResponse.Members[index]
		if member.UserID == trimmed ||
			strings.EqualFold(stringOrEmpty(member.GithubLogin), strings.TrimPrefix(trimmed, "@")) ||
			strings.EqualFold(stringOrEmpty(member.Email), trimmed) ||
			strings.EqualFold(stringOrEmpty(member.Name), trimmed) {
			return &listResponse.Members[index], nil
		}
	}
	names := make([]string, 0, len(listResponse.Members))
	for _, member := range listResponse.Members {
		names = append(names, memberName(member))
	}
	return nil, withExitCode(exitUsage, fmt.Errorf("nobody called %q in this workspace; members: %s", trimmed, strings.Join(names, ", ")))
}
