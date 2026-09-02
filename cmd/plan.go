package cmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"tael.io/cli/internal/client"
)

// tael plan does two things by what it is given. With a sentence it asks
// Tael to plan a change to the workspace (see architecture.go); without
// one it shows the workspace's plan: the tier, what it holds, the runtime
// and any coupon in force.
var planCmd = &cobra.Command{
	Use:   "plan [\"<what you want>\"] [--app <app> | --stack <stack>] [--build] [--yes]",
	Short: "Ask Tael to plan a change in your words; alone, the workspace's plan and coupon",
	Long: `With a sentence, ask Tael to plan a change: "Add a database for web",
"Connect api to the object storage". Tael answers with the changes it
would make, kept as the last plan for ` + "`tael build`" + `; nothing runs until
you say so. --app or --stack scopes the plan to that slice of the
workspace. --build carries the plan out straight away, after asking.

Without a sentence, the workspace's plan: the tier, what it holds, the
runtime, and any coupon in force.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(command *cobra.Command, args []string) error {
		if len(args) > 0 || planBuildFlag || planAppFlag != "" || planStackFlag != "" {
			return runArchitecturePlan(command, args)
		}
		status, statusError := apiClient.GetWorkspaceStatus(command.Context())
		if statusError != nil {
			return statusError
		}
		coupon, couponError := apiClient.GetCouponGrant(command.Context())
		if couponError != nil {
			return couponError
		}
		asJSON, formatError := wantsJSON(planJSONFlag)
		if formatError != nil {
			return formatError
		}
		if asJSON {
			return writeJSON(command, map[string]any{"status": status, "coupon": coupon})
		}
		fmt.Fprint(command.OutOrStdout(), renderPlan(status, coupon.Grant))
		return nil
	},
}

// tael coupon with a code applies it; alone, it shows the coupon in force.
var couponCmd = &cobra.Command{
	Use:   "coupon [code]",
	Short: "Apply a coupon code to the workspace; alone, the coupon in force",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(command *cobra.Command, args []string) error {
		if len(args) == 0 {
			return runCouponShow(command)
		}
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

// runCouponShow prints the coupon in force: the applied coupon when the
// API reports one, else the grant in the older words, else none.
func runCouponShow(command *cobra.Command) error {
	response, couponError := apiClient.GetCouponGrant(command.Context())
	if couponError != nil {
		return couponError
	}
	if rendered, renderError := renderJSON(command, response); rendered || renderError != nil {
		return renderError
	}
	switch {
	case response.Applied != nil:
		fmt.Fprintln(command.OutOrStdout(), appliedCouponLine(response.Applied))
	case response.Grant != nil:
		fmt.Fprintf(command.OutOrStdout(), "%s — %s\n", response.Grant.Code, grantWords(response.Grant))
	default:
		fmt.Fprintln(command.OutOrStdout(), "No coupon in force. Have one? `tael coupon <code>`.")
	}
	return nil
}

// appliedCouponLine is the one line for a coupon in force:
// "TAEL-XXXX applied — Launch until 28 Feb 2027 · 5 apps · 20M AI tokens".
func appliedCouponLine(applied *client.AppliedCoupon) string {
	parts := []string{fmt.Sprintf("%s until %s", planName(applied.Plan), formatDate(applied.GrantedUntil))}
	if applied.AppsIncluded > 0 {
		parts = append(parts, plural(applied.AppsIncluded, "app", "apps"))
	}
	if applied.AITokensIncluded > 0 {
		parts = append(parts, compactCount(applied.AITokensIncluded)+" AI tokens")
	}
	return fmt.Sprintf("%s applied — %s", applied.Code, strings.Join(parts, " · "))
}

// planName is the plan as a name: launch → Launch.
func planName(plan string) string {
	plan = strings.TrimSpace(plan)
	if plan == "" {
		return "-"
	}
	return strings.ToUpper(plan[:1]) + plan[1:]
}

// formatDate renders a day the way a sentence says it, "28 Feb 2027",
// from a date or an RFC3339 timestamp; anything else is shown as sent.
func formatDate(raw string) string {
	for _, layout := range []string{time.RFC3339, "2006-01-02"} {
		if parsed, parseError := time.Parse(layout, raw); parseError == nil {
			return parsed.Format("2 Jan 2006")
		}
	}
	return valueOrDash(raw)
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

// runtimeNotAnswering says the backend reports the runtime as not
// answering right now; an older backend says nothing, which reads as fine.
func runtimeNotAnswering(status *client.WorkspaceStatus) bool {
	return status.Runtime != nil && !status.Runtime.Reachable
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
		if runtimeNotAnswering(status) {
			runtime = "not answering"
		}
		if environment.Tier != nil && *environment.Tier != "" {
			runtime += " (" + *environment.Tier + ")"
		}
		if environment.BaseDomain != nil && *environment.BaseDomain != "" {
			runtime += " · addresses under " + *environment.BaseDomain
		}
		fmt.Fprintf(&builder, "Runtime:   %s\n", runtime)
		if runtimeNotAnswering(status) && status.Runtime.Detail != "" {
			fmt.Fprintf(&builder, "           %s\n", status.Runtime.Detail)
		}
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
