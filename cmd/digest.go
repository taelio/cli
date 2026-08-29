package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"tael.io/cli/internal/client"
)

var digestDaysFlag int

var digestCmd = &cobra.Command{
	Use:   "digest [--days 7]",
	Short: "What happened over the last days, in Tael's words, with the numbers",
	Args:  cobra.NoArgs,
	RunE: func(command *cobra.Command, _ []string) error {
		digest, digestError := apiClient.GetDigest(command.Context(), digestDaysFlag)
		if digestError != nil {
			return digestError
		}
		if rendered, renderError := renderJSON(command, digest); rendered || renderError != nil {
			return renderError
		}
		fmt.Fprint(command.OutOrStdout(), renderDigest(digest))
		return nil
	},
}

func init() {
	digestCmd.Flags().IntVar(&digestDaysFlag, "days", 0, "how many days back to read (the API's default when unset)")
	rootCmd.AddCommand(digestCmd)
}

func plural(count int, singular string, pluralForm string) string {
	if count == 1 {
		return fmt.Sprintf("%d %s", count, singular)
	}
	return fmt.Sprintf("%d %s", count, pluralForm)
}

// renderDigest prints the reading — headline, what changed, what broke,
// what needs you — then the numbers line the reading was written from.
// While Tael is still writing, it says so and prints the numbers.
func renderDigest(digest *client.Digest) string {
	var builder strings.Builder
	facts := digest.Facts
	switch {
	case digest.Prose != nil:
		prose := digest.Prose
		if headline := strings.TrimSpace(prose.Headline); headline != "" {
			builder.WriteString(headline + "\n")
		}
		if changed := strings.TrimSpace(prose.WhatChanged); changed != "" {
			builder.WriteString("What changed: " + changed + "\n")
		}
		if broke := strings.TrimSpace(prose.WhatBroke); broke != "" {
			builder.WriteString("What broke: " + broke + "\n")
		}
		if len(prose.NeedsYou) > 0 {
			builder.WriteString("Needs you:\n")
			for _, item := range prose.NeedsYou {
				builder.WriteString("  - " + strings.TrimSpace(item) + "\n")
			}
		}
	case digest.Writing:
		builder.WriteString("Tael is still writing the reading; ask again in a moment. The numbers are exact already.\n")
	default:
		builder.WriteString("No reading written for this window; here are the numbers.\n")
	}
	if digest.Prose == nil {
		// Without prose the facts carry what needs saying.
		for _, failed := range facts.FailedDeploys {
			line := fmt.Sprintf("Failed deploy: %s", failed.App)
			if failed.When != "" {
				line += " " + failed.When
			}
			if failed.Error != "" {
				line += " — " + failed.Error
			}
			builder.WriteString(line + "\n")
		}
		for _, incident := range facts.OpenIncidents {
			builder.WriteString(fmt.Sprintf("Open incident: %s — %s (%s, open for %s)\n", incident.App, incident.Title, incident.Severity, incident.OpenFor))
		}
		if len(facts.NeedsYou) > 0 {
			builder.WriteString("Needs you:\n")
			for _, approval := range facts.NeedsYou {
				line := "  - " + approval.Ask
				if approval.App != "" {
					line += " (" + approval.App + ")"
				}
				builder.WriteString(line + "\n")
			}
		}
	}
	builder.WriteString(renderDigestNumbers(facts) + "\n")
	return builder.String()
}

// renderDigestNumbers is the one line of exact numbers.
func renderDigestNumbers(facts client.DigestFacts) string {
	parts := []string{}
	deploys := plural(facts.DeploysTotal, "deploy", "deploys")
	if facts.DeploysTotal > 0 {
		deploys += fmt.Sprintf(" (%d succeeded, %d failed)", facts.DeploysSucceeded, facts.DeploysFailed)
		if len(facts.AppsDeployed) > 0 {
			deploys += " across " + strings.Join(facts.AppsDeployed, ", ")
		}
	}
	parts = append(parts, deploys)
	parts = append(parts, fmt.Sprintf("%s opened, %d resolved", plural(facts.IncidentsOpened, "incident", "incidents"), facts.IncidentsResolved))
	if len(facts.OpenIncidents) > 0 {
		parts = append(parts, fmt.Sprintf("%d still open", len(facts.OpenIncidents)))
	}
	if len(facts.NeedsYou) > 0 {
		parts = append(parts, fmt.Sprintf("%s on you", plural(len(facts.NeedsYou), "decision", "decisions")))
	}
	if len(facts.OpenSuggestions) > 0 {
		parts = append(parts, plural(len(facts.OpenSuggestions), "suggestion", "suggestions"))
	}
	if facts.NewMembers > 0 {
		parts = append(parts, plural(facts.NewMembers, "new member", "new members"))
	}
	window := fmt.Sprintf("last %s", plural(facts.WindowDays, "day", "days"))
	return "Numbers: " + strings.Join(parts, " · ") + " · " + window
}
