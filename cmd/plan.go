package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"tael.io/cli/internal/client"
)

var planCmd = &cobra.Command{
	Use:   "plan",
	Short: "The workspace's plan, what it holds, and any coupon in force",
	Args:  cobra.NoArgs,
	RunE: func(command *cobra.Command, _ []string) error {
		status, statusError := apiClient.GetWorkspaceStatus(command.Context())
		if statusError != nil {
			return statusError
		}
		coupon, couponError := apiClient.GetCouponGrant(command.Context())
		if couponError != nil {
			return couponError
		}
		if rendered, renderError := renderJSON(command, map[string]any{"status": status, "coupon": coupon}); rendered || renderError != nil {
			return renderError
		}
		fmt.Fprint(command.OutOrStdout(), renderPlan(status, coupon.Grant))
		return nil
	},
}

var couponCmd = &cobra.Command{
	Use:   "coupon <code>",
	Short: "Apply a coupon code to the workspace",
	Args:  cobra.ExactArgs(1),
	RunE: func(command *cobra.Command, args []string) error {
		code := strings.TrimSpace(args[0])
		if code == "" {
			return withExitCode(exitUsage, fmt.Errorf("say the code"))
		}
		response, redeemError := apiClient.RedeemCoupon(command.Context(), code)
		if redeemError != nil {
			return redeemError
		}
		if rendered, renderError := renderJSON(command, response); rendered || renderError != nil {
			return renderError
		}
		if response.Grant == nil {
			fmt.Fprintln(command.OutOrStdout(), "Applied.")
			return nil
		}
		fmt.Fprintf(command.OutOrStdout(), "Applied %s: %s\n", response.Grant.Code, grantWords(response.Grant))
		return nil
	},
}

func init() {
	rootCmd.AddCommand(planCmd, couponCmd)
}

// grantWords says what a coupon put in place.
func grantWords(grant *client.CouponGrant) string {
	words := fmt.Sprintf("this workspace is on %s until %s", valueOrDash(grant.Plan), formatTimestamp(grant.Until))
	var included []string
	if grant.AppsIncluded > 0 {
		included = append(included, plural(grant.AppsIncluded, "app", "apps"))
	}
	if grant.AITokensIncluded > 0 {
		included = append(included, fmt.Sprintf("%s AI tokens", withThousands(grant.AITokensIncluded)))
	}
	if len(included) > 0 {
		words += " (" + strings.Join(included, ", ") + " included)"
	}
	return words + "."
}

func withThousands(value int64) string {
	digits := fmt.Sprintf("%d", value)
	var builder strings.Builder
	for index, digit := range digits {
		if index > 0 && (len(digits)-index)%3 == 0 {
			builder.WriteByte(',')
		}
		builder.WriteRune(digit)
	}
	return builder.String()
}

// renderPlan prints the plan and what the workspace holds, the runtime,
// and the coupon in force.
func renderPlan(status *client.WorkspaceStatus, grant *client.CouponGrant) string {
	var builder strings.Builder
	fmt.Fprintf(&builder, "Plan:      %s\n", valueOrDash(status.Plan))
	holding := []string{fmt.Sprintf("%s, %d live", plural(status.AppsTotal, "app", "apps"), status.AppsLive)}
	holding = append(holding, plural(status.OpenIncidents, "open incident", "open incidents"))
	if status.NeedsYou > 0 {
		holding = append(holding, plural(status.NeedsYou, "decision on you", "decisions on you"))
	}
	fmt.Fprintf(&builder, "Holding:   %s\n", strings.Join(holding, " · "))
	if environment := status.Environment; environment != nil {
		runtime := valueOrDash(environment.Status)
		if environment.Tier != nil && *environment.Tier != "" {
			runtime += " (" + *environment.Tier + ")"
		}
		if environment.BaseDomain != nil && *environment.BaseDomain != "" {
			runtime += " · addresses under " + *environment.BaseDomain
		}
		fmt.Fprintf(&builder, "Runtime:   %s\n", runtime)
		if environment.PendingRuntime != nil && *environment.PendingRuntime != "" {
			builder.WriteString("           A dedicated runtime is on its way; apps move over when it is ready.\n")
		}
		if environment.UpgradeError != nil && *environment.UpgradeError != "" {
			fmt.Fprintf(&builder, "           The move to a dedicated runtime hit a problem: %s\n", *environment.UpgradeError)
		}
	} else if status.RuntimeStatus != nil && *status.RuntimeStatus != "" {
		fmt.Fprintf(&builder, "Runtime:   %s\n", *status.RuntimeStatus)
	}
	if grant == nil {
		builder.WriteString("Coupon:    none. Have one? `tael coupon <code>`.\n")
	} else {
		fmt.Fprintf(&builder, "Coupon:    %s — %s\n", grant.Code, grantWords(grant))
	}
	return builder.String()
}
