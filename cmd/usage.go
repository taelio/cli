package cmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"tael.io/cli/internal/client"
)

// tael usage shows the meters: what the workspace has used this period
// against what its plan or coupon includes.
var usageCmd = &cobra.Command{
	Use:   "usage",
	Short: "What the workspace has used this period, against what its plan includes",
	Long: `The meters for this period: apps, seats, AI tokens, custom domains, and
the deploys and builds so far. An allowance that comes from a coupon says
so; when part of the AI figure is estimated rather than reported, the line
says that too.`,
	Args: cobra.NoArgs,
	RunE: func(command *cobra.Command, _ []string) error {
		summary, usageError := apiClient.GetUsage(command.Context())
		if usageError != nil {
			return usageError
		}
		if rendered, renderError := renderJSON(command, summary); rendered || renderError != nil {
			return renderError
		}
		fmt.Fprint(command.OutOrStdout(), renderUsage(summary))
		return nil
	},
}

func init() {
	rootCmd.AddCommand(usageCmd)
}

// renderUsage prints the period and one line per meter. A meter the API
// did not send is left out rather than shown as zero.
func renderUsage(summary *client.UsageSummary) string {
	var builder strings.Builder
	if period := formatUsagePeriod(summary.Period); period != "" {
		builder.WriteString(meterLine("Period", period))
	}
	if apps := summary.Apps; apps != nil {
		builder.WriteString(meterLine("Apps", usedOfIncluded(apps.Used, apps.Included, withThousands)))
	}
	if seats := summary.Seats; seats != nil {
		builder.WriteString(meterLine("Seats", usedOfIncluded(seats.Used, seats.Included, withThousands)))
	}
	if tokens := summary.AITokens; tokens != nil {
		line := usedOfIncluded(tokens.Used, tokens.Included, compactCount)
		if tokens.Source == "coupon" {
			line += " (coupon)"
		}
		if tokens.Estimated > 0 {
			line += " · part estimated"
		}
		builder.WriteString(meterLine("AI tokens", line))
	}
	if domains := summary.CustomDomains; domains != nil {
		builder.WriteString(meterLine("Custom domains", customDomainsWords(domains)))
	}
	if summary.Deploys != nil {
		builder.WriteString(meterLine("Deploys", withThousands(*summary.Deploys)+" this period"))
	}
	if summary.Builds != nil {
		builder.WriteString(meterLine("Builds", withThousands(*summary.Builds)+" this period"))
	}
	if summary.Machines != nil && *summary.Machines > 0 {
		builder.WriteString(meterLine("Machines", withThousands(*summary.Machines)))
	}
	if builder.Len() == 0 {
		builder.WriteString("Nothing counted yet this period.\n")
	}
	return builder.String()
}

// meterLine is a label padded to a column, then the value; a label longer
// than the column keeps two spaces so it never runs into its value.
func meterLine(label string, value string) string {
	padding := 12 - len(label)
	if padding < 2 {
		padding = 2
	}
	return label + strings.Repeat(" ", padding) + value + "\n"
}

// usedOfIncluded says "3 of 5", or just "3" when no allowance was given.
func usedOfIncluded(used int64, included int64, format func(int64) string) string {
	if included <= 0 {
		return format(used)
	}
	return format(used) + " of " + format(included)
}

// customDomainsWords says whether custom domains are open to the
// workspace: on the free plan they come with Launch; on a paid plan the
// line says how many apps have one.
func customDomainsWords(domains *client.UsageCustomDomains) string {
	if !domains.Allowed {
		return "on Launch"
	}
	if domains.Used <= 0 {
		return "yes"
	}
	return fmt.Sprintf("yes · %d in use", domains.Used)
}

// formatUsagePeriod turns the period's first day into its month: "August
// 2026". Anything that does not parse is shown as sent; empty stays empty.
func formatUsagePeriod(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	for _, layout := range []string{"2006-01-02", time.RFC3339, "2006-01"} {
		if parsed, parseError := time.Parse(layout, raw); parseError == nil {
			return parsed.Format("January 2006")
		}
	}
	return raw
}

// compactCount writes a token count the way people say it: 850, 200k,
// 1.2M, 20M. One decimal at most, and none when it would be zero.
func compactCount(value int64) string {
	switch {
	case value >= 1_000_000:
		return trimDecimal(float64(value)/1_000_000) + "M"
	case value >= 1_000:
		return trimDecimal(float64(value)/1_000) + "k"
	default:
		return fmt.Sprintf("%d", value)
	}
}

func trimDecimal(value float64) string {
	return strings.TrimSuffix(fmt.Sprintf("%.1f", value), ".0")
}
