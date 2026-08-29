package cmd

import (
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"tael.io/cli/internal/client"
)

// tael invite — make an invitation: a link anyone can use, or one
// addressed to an email or a GitHub login. tael invites lists and revokes.

var (
	inviteRoleFlag    string
	inviteMaxUsesFlag int
)

var inviteCmd = &cobra.Command{
	Use:   "invite",
	Short: "Invite someone: by link, by email, or by GitHub login (owners and admins only)",
}

var inviteLinkCmd = &cobra.Command{
	Use:   "link",
	Short: "Make a join link anyone can use",
	Args:  cobra.NoArgs,
	RunE: func(command *cobra.Command, _ []string) error {
		return createInvitation(command, client.NewInvitation{Kind: "link", Role: inviteRoleFlag, MaxUses: inviteMaxUsesFlag})
	},
}

var inviteEmailCmd = &cobra.Command{
	Use:   "email <address>",
	Short: "Invite the person with this email address",
	Args:  cobra.ExactArgs(1),
	RunE: func(command *cobra.Command, args []string) error {
		return createInvitation(command, client.NewInvitation{Kind: "email", Role: inviteRoleFlag, Email: args[0]})
	},
}

var inviteGithubCmd = &cobra.Command{
	Use:   "github <login>",
	Short: "Invite the person with this GitHub login",
	Args:  cobra.ExactArgs(1),
	RunE: func(command *cobra.Command, args []string) error {
		return createInvitation(command, client.NewInvitation{Kind: "github", Role: inviteRoleFlag, GithubLogin: strings.TrimPrefix(args[0], "@")})
	},
}

var invitesCmd = &cobra.Command{
	Use:   "invites",
	Short: "List the workspace's invitations; `invites revoke <id>` stops one working",
	Args:  cobra.NoArgs,
	RunE: func(command *cobra.Command, _ []string) error {
		listResponse, listError := apiClient.ListInvitations(command.Context())
		if listError != nil {
			return listError
		}
		if rendered, renderError := renderJSON(command, listResponse); rendered || renderError != nil {
			return renderError
		}
		out := command.OutOrStdout()
		if len(listResponse.Invitations) == 0 {
			fmt.Fprintln(out, "No invitations. Make one with `tael invite link`, `tael invite email <address>` or `tael invite github <login>`.")
			return nil
		}
		fmt.Fprint(out, renderInvitationsTable(listResponse.Invitations))
		return nil
	},
}

var invitesRevokeCmd = &cobra.Command{
	Use:   "revoke <id>",
	Short: "Stop an invitation working",
	Args:  cobra.ExactArgs(1),
	RunE: func(command *cobra.Command, args []string) error {
		if revokeError := apiClient.RevokeInvitation(command.Context(), args[0]); revokeError != nil {
			return revokeError
		}
		if rendered, renderError := renderJSON(command, map[string]string{"id": args[0], "status": "revoked"}); rendered || renderError != nil {
			return renderError
		}
		fmt.Fprintf(command.OutOrStdout(), "Revoked. Anyone holding that invitation can no longer use it.\n")
		return nil
	},
}

func init() {
	inviteCmd.PersistentFlags().StringVar(&inviteRoleFlag, "role", "", "the role to join with: member (the default) or admin")
	inviteLinkCmd.Flags().IntVar(&inviteMaxUsesFlag, "max-uses", 0, "how many people may use the link (0 for no limit)")
	inviteCmd.AddCommand(inviteLinkCmd, inviteEmailCmd, inviteGithubCmd)
	invitesCmd.AddCommand(invitesRevokeCmd)
	rootCmd.AddCommand(inviteCmd, invitesCmd)
}

func createInvitation(command *cobra.Command, invitation client.NewInvitation) error {
	if invitation.Role != "" && invitation.Role != "member" && invitation.Role != "admin" {
		return withExitCode(exitUsage, fmt.Errorf("--role takes member or admin, not %q", invitation.Role))
	}
	created, createError := apiClient.CreateInvitation(command.Context(), invitation)
	if createError != nil {
		return createError
	}
	if rendered, renderError := renderJSON(command, created); rendered || renderError != nil {
		return renderError
	}
	fmt.Fprint(command.OutOrStdout(), renderCreatedInvitation(created))
	return nil
}

// invitationTo says who an invitation is for.
func invitationTo(invitation client.Invitation) string {
	switch invitation.Kind {
	case "email":
		return stringOrEmpty(invitation.Email)
	case "github":
		return "@" + stringOrEmpty(invitation.GithubLogin)
	}
	return "anyone with the link"
}

// renderCreatedInvitation prints the join link once, with what it does.
func renderCreatedInvitation(created *client.CreatedInvitation) string {
	var builder strings.Builder
	invitation := created.Invitation
	role := invitation.Role
	if role == "" {
		role = "member"
	}
	fmt.Fprintf(&builder, "Invited %s as %s.\n", invitationTo(invitation), role)
	fmt.Fprintf(&builder, "Join link: %s\n", created.JoinURL)
	builder.WriteString("This is the only time the link is shown. ")
	if invitation.Kind == "link" {
		if invitation.MaxUses != nil && *invitation.MaxUses > 0 {
			fmt.Fprintf(&builder, "It works %d time(s) and ", *invitation.MaxUses)
		} else {
			builder.WriteString("It ")
		}
	} else {
		builder.WriteString("It only works for them and ")
	}
	fmt.Fprintf(&builder, "expires %s. Lose it and revoke it with `tael invites revoke %s`.\n", formatTimestamp(invitation.ExpiresAt), invitation.ID)
	return builder.String()
}

// renderInvitationsTable renders the invitations as an aligned table.
func renderInvitationsTable(invitations []client.Invitation) string {
	var builder strings.Builder
	table := tabwriter.NewWriter(&builder, 0, 0, 2, ' ', 0)
	fmt.Fprintln(table, "ID\tTO\tROLE\tSTATUS\tUSED\tEXPIRES")
	for _, invitation := range invitations {
		used := fmt.Sprintf("%d", invitation.Uses)
		if invitation.MaxUses != nil && *invitation.MaxUses > 0 {
			used = fmt.Sprintf("%d of %d", invitation.Uses, *invitation.MaxUses)
		}
		fmt.Fprintf(table, "%s\t%s\t%s\t%s\t%s\t%s\n",
			invitation.ID,
			invitationTo(invitation),
			valueOrDash(invitation.Role),
			valueOrDash(invitation.Status),
			used,
			valueOrDash(formatTimestamp(invitation.ExpiresAt)),
		)
	}
	_ = table.Flush()
	return builder.String()
}
