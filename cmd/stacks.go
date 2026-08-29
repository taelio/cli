package cmd

import (
	"context"
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"tael.io/cli/internal/client"
)

// Stacks in the terminal. A stack is a named group of apps that ship
// together; an app belongs to at most one. `tael stacks` lists them,
// `tael stack` makes, moves, renames and removes, and `tael link` /
// `tael unlink` declare that one app calls another — lines the picture
// draws and the planner reads, nothing that runs.

var stacksCmd = &cobra.Command{
	Use:   "stacks",
	Short: "List the stacks: named groups of apps that ship together",
	Args:  cobra.NoArgs,
	RunE: func(command *cobra.Command, _ []string) error {
		listResponse, listError := apiClient.ListStacks(command.Context())
		if listError != nil {
			return listError
		}
		if rendered, renderError := renderJSON(command, listResponse); rendered || renderError != nil {
			return renderError
		}
		if len(listResponse.Stacks) == 0 {
			fmt.Fprintln(command.OutOrStdout(), "No stacks yet. Group apps with `tael stack new <name> --app <app>`.")
			return nil
		}
		fmt.Fprint(command.OutOrStdout(), renderStacksTable(listResponse.Stacks))
		return nil
	},
}

var stackCmd = &cobra.Command{
	Use:   "stack",
	Short: "Group apps into stacks: make one, move apps in and out, rename, remove",
	Long: `A stack is a named group of apps that ship together; an app belongs to at
most one. The workspace picture shows each stack as one card with its apps
inside. Removing a stack leaves its apps in place, ungrouped.`,
}

var stackNewAppFlags []string

var stackNewCmd = &cobra.Command{
	Use:   "new <name> [--app <app> ...]",
	Short: "Make a stack, with apps in it from the start",
	Args:  cobra.ExactArgs(1),
	RunE: func(command *cobra.Command, args []string) error {
		name := strings.TrimSpace(args[0])
		if name == "" {
			return withExitCode(exitUsage, fmt.Errorf("say the stack's name: tael stack new <name>"))
		}
		appIDs := make([]string, 0, len(stackNewAppFlags))
		seen := map[string]bool{}
		for _, word := range stackNewAppFlags {
			appID, resolveError := resolveAppID(command.Context(), word)
			if resolveError != nil {
				return resolveError
			}
			if seen[appID] {
				continue
			}
			seen[appID] = true
			appIDs = append(appIDs, appID)
		}
		stack, createError := apiClient.CreateStack(command.Context(), client.CreateStackRequest{Name: name, AppIDs: appIDs})
		if createError != nil {
			return createError
		}
		if rendered, renderError := renderJSON(command, stack); rendered || renderError != nil {
			return renderError
		}
		out := command.OutOrStdout()
		if len(appIDs) == 0 {
			fmt.Fprintf(out, "Made the stack %s. Put apps in it with `tael stack move <app> %s`.\n", name, name)
			return nil
		}
		fmt.Fprintf(out, "Made the stack %s with %s. See it with `tael architecture --stack %s`.\n", name, plural(len(appIDs), "app", "apps"), name)
		return nil
	},
}

var stackMoveNoneFlag bool

var stackMoveCmd = &cobra.Command{
	Use:   "move <app> <stack>",
	Short: "Move an app into a stack; --none puts it on its own again",
	Args:  cobra.RangeArgs(1, 2),
	RunE: func(command *cobra.Command, args []string) error {
		if stackMoveNoneFlag && len(args) > 1 {
			return withExitCode(exitUsage, fmt.Errorf("--none takes only the app: tael stack move <app> --none"))
		}
		if !stackMoveNoneFlag && len(args) < 2 {
			return withExitCode(exitUsage, fmt.Errorf("say the stack: tael stack move <app> <stack>, or --none to ungroup"))
		}
		appID, resolveError := resolveAppID(command.Context(), args[0])
		if resolveError != nil {
			return resolveError
		}
		var stackID *string
		var stackName string
		if !stackMoveNoneFlag {
			stack, stackError := resolveStackArgument(command.Context(), args[1])
			if stackError != nil {
				return stackError
			}
			stackID = &stack.ID
			stackName = stack.Name
		}
		if moveError := apiClient.MoveAppToStack(command.Context(), appID, stackID); moveError != nil {
			return moveError
		}
		if rendered, renderError := renderJSON(command, map[string]any{"app_id": appID, "stack_id": stackID}); rendered || renderError != nil {
			return renderError
		}
		if stackID == nil {
			fmt.Fprintf(command.OutOrStdout(), "Moved %s out of its stack; it stands on its own again.\n", args[0])
			return nil
		}
		fmt.Fprintf(command.OutOrStdout(), "Moved %s into %s.\n", args[0], stackName)
		return nil
	},
}

var stackRenameCmd = &cobra.Command{
	Use:   "rename <stack> <name>",
	Short: "Give a stack a new name",
	Args:  cobra.ExactArgs(2),
	RunE: func(command *cobra.Command, args []string) error {
		name := strings.TrimSpace(args[1])
		if name == "" {
			return withExitCode(exitUsage, fmt.Errorf("say the new name: tael stack rename <stack> <name>"))
		}
		stack, resolveError := resolveStackArgument(command.Context(), args[0])
		if resolveError != nil {
			return resolveError
		}
		renamed, renameError := apiClient.PatchStack(command.Context(), stack.ID, client.PatchStackRequest{Name: name})
		if renameError != nil {
			return renameError
		}
		if rendered, renderError := renderJSON(command, renamed); rendered || renderError != nil {
			return renderError
		}
		fmt.Fprintf(command.OutOrStdout(), "Renamed %s to %s.\n", stack.Name, name)
		return nil
	},
}

var stackRemoveYesFlag bool

var stackRemoveCmd = &cobra.Command{
	Use:   "remove <stack> [--yes]",
	Short: "Remove a stack; its apps stay, ungrouped",
	Args:  cobra.ExactArgs(1),
	RunE: func(command *cobra.Command, args []string) error {
		stack, resolveError := resolveStackArgument(command.Context(), args[0])
		if resolveError != nil {
			return resolveError
		}
		if !stackRemoveYesFlag {
			if !stdinIsTerminal() {
				return withExitCode(exitUsage, fmt.Errorf(
					"removing the stack %s leaves its apps ungrouped; not a terminal, so run again with --yes", stack.Name))
			}
			question := fmt.Sprintf("Remove the stack %s? It holds no apps. [y/N] ", stack.Name)
			if held := len(stack.Apps); held > 0 {
				question = fmt.Sprintf("Remove the stack %s? Its %s stay, ungrouped. [y/N] ", stack.Name, plural(held, "app", "apps"))
			}
			confirmed, confirmError := confirm(command.InOrStdin(), command.OutOrStdout(), question)
			if confirmError != nil {
				return confirmError
			}
			if !confirmed {
				fmt.Fprintln(command.OutOrStdout(), "Nothing removed.")
				return nil
			}
		}
		if removeError := apiClient.DeleteStack(command.Context(), stack.ID); removeError != nil {
			return removeError
		}
		if rendered, renderError := renderJSON(command, map[string]string{"id": stack.ID, "status": "removed"}); rendered || renderError != nil {
			return renderError
		}
		if len(stack.Apps) > 0 {
			fmt.Fprintf(command.OutOrStdout(), "Removed the stack %s. Its %s stay, ungrouped.\n", stack.Name, plural(len(stack.Apps), "app", "apps"))
			return nil
		}
		fmt.Fprintf(command.OutOrStdout(), "Removed the stack %s.\n", stack.Name)
		return nil
	},
}

var linkLabelFlag string

var linkCmd = &cobra.Command{
	Use:   "link <from-app> <to-app> [--label <text>]",
	Short: "Say one app calls another; the picture draws it and the planner knows",
	Long: `Declare that one app calls another — "web calls api". The line is drawn in
the picture and handed to the planner as context; nothing runs because of
it. --label says how, in a word: REST, gRPC, a queue.`,
	Args: cobra.ExactArgs(2),
	RunE: func(command *cobra.Command, args []string) error {
		fromID, toID, resolveError := resolveLinkEnds(command.Context(), args[0], args[1])
		if resolveError != nil {
			return resolveError
		}
		link, linkError := apiClient.CreateArchitectureLink(command.Context(), fromID, toID, linkLabelFlag)
		if linkError != nil {
			return linkError
		}
		if rendered, renderError := renderJSON(command, link); rendered || renderError != nil {
			return renderError
		}
		sentence := fmt.Sprintf("%s now calls %s", args[0], args[1])
		if linkLabelFlag != "" {
			sentence += " (" + linkLabelFlag + ")"
		}
		fmt.Fprintf(command.OutOrStdout(), "%s. The picture shows it; take it back with `tael unlink %s %s`.\n", sentence, args[0], args[1])
		return nil
	},
}

var unlinkCmd = &cobra.Command{
	Use:   "unlink <from-app> <to-app>",
	Short: "Take back that one app calls another",
	Args:  cobra.ExactArgs(2),
	RunE: func(command *cobra.Command, args []string) error {
		fromID, toID, resolveError := resolveLinkEnds(command.Context(), args[0], args[1])
		if resolveError != nil {
			return resolveError
		}
		if unlinkError := apiClient.DeleteArchitectureLink(command.Context(), fromID, toID); unlinkError != nil {
			return unlinkError
		}
		if rendered, renderError := renderJSON(command, map[string]string{"from_app_id": fromID, "to_app_id": toID, "status": "removed"}); rendered || renderError != nil {
			return renderError
		}
		fmt.Fprintf(command.OutOrStdout(), "%s no longer calls %s.\n", args[0], args[1])
		return nil
	},
}

func init() {
	stackNewCmd.Flags().StringArrayVar(&stackNewAppFlags, "app", nil, "an app to put in the stack (name or id; repeatable)")
	stackMoveCmd.Flags().BoolVar(&stackMoveNoneFlag, "none", false, "take the app out of its stack instead")
	stackRemoveCmd.Flags().BoolVar(&stackRemoveYesFlag, "yes", false, "do not ask before removing")
	linkCmd.Flags().StringVar(&linkLabelFlag, "label", "", "how the call is made, in a word: REST, gRPC, a queue")
	stackCmd.AddCommand(stackNewCmd, stackMoveCmd, stackRenameCmd, stackRemoveCmd)
	rootCmd.AddCommand(stacksCmd, stackCmd, linkCmd, unlinkCmd)
}

// renderStacksTable renders the stacks as an aligned text table: the name,
// how many apps, and who they are.
func renderStacksTable(stacks []client.Stack) string {
	var builder strings.Builder
	table := tabwriter.NewWriter(&builder, 0, 0, 2, ' ', 0)
	fmt.Fprintln(table, "NAME\tAPPS\tMEMBERS")
	for _, stack := range stacks {
		names := make([]string, 0, len(stack.Apps))
		for _, member := range stack.Apps {
			names = append(names, member.Name)
		}
		fmt.Fprintf(table, "%s\t%d\t%s\n", stack.Name, len(stack.Apps), valueOrDash(strings.Join(names, ", ")))
	}
	_ = table.Flush()
	return builder.String()
}

// resolveStackArgument finds the stack the person named, by id or by name.
// An unknown one lists what there is.
func resolveStackArgument(requestContext context.Context, word string) (*client.Stack, error) {
	trimmed := strings.TrimSpace(word)
	if trimmed == "" {
		return nil, withExitCode(exitUsage, fmt.Errorf("say which stack (name or id)"))
	}
	listResponse, listError := apiClient.ListStacks(requestContext)
	if listError != nil {
		return nil, listError
	}
	for index := range listResponse.Stacks {
		stack := &listResponse.Stacks[index]
		if stack.ID == trimmed || strings.EqualFold(stack.Name, trimmed) {
			return stack, nil
		}
	}
	if len(listResponse.Stacks) == 0 {
		return nil, withExitCode(exitUsage, fmt.Errorf("no stacks yet; make one with `tael stack new <name>`"))
	}
	names := make([]string, 0, len(listResponse.Stacks))
	for _, stack := range listResponse.Stacks {
		names = append(names, stack.Name)
	}
	return nil, withExitCode(exitUsage, fmt.Errorf("no stack called %q; stacks: %s", trimmed, strings.Join(names, ", ")))
}

// resolveLinkEnds resolves both apps of a link and refuses an app calling
// itself before the API has to.
func resolveLinkEnds(requestContext context.Context, fromWord string, toWord string) (string, string, error) {
	fromID, fromError := resolveAppID(requestContext, fromWord)
	if fromError != nil {
		return "", "", fromError
	}
	toID, toError := resolveAppID(requestContext, toWord)
	if toError != nil {
		return "", "", toError
	}
	if fromID == toID {
		return "", "", withExitCode(exitUsage, fmt.Errorf("an app does not call itself; name two apps"))
	}
	return fromID, toID, nil
}
