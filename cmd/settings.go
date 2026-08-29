package cmd

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"tael.io/cli/internal/client"
)

// tael settings ai — how much Tael may do on its own: who may approve,
// what runs unasked and how often, and when scheduled work keeps quiet.

var (
	settingsApproversFlag       string
	settingsPreApproveFlags     []string
	settingsQuietHoursFlag      string
	settingsClearQuietHoursFlag bool
)

var settingsCmd = &cobra.Command{
	Use:   "settings",
	Short: "Workspace settings",
}

var settingsAICmd = &cobra.Command{
	Use:   "ai [--approvers admins|members] [--pre-approve category=N] [--quiet-hours HH:MM-HH:MM[@Zone]] [--clear-quiet-hours]",
	Short: "How much Tael may do on its own: who approves, what runs unasked, quiet hours",
	Long: `Show how much Tael may do in this workspace, or change it (owners and
admins only). --approvers says who may say yes to what Tael asks: admins
(owners and admins) or members (everyone). --pre-approve lets a category
run without asking, up to N times a month (0 asks again); the categories
are restart, scale, rollback, config_change, open_pr, install_solution and
other. --quiet-hours keeps scheduled work quiet between two times in a
zone (UTC when none is given); --clear-quiet-hours lifts it.`,
	Args: cobra.NoArgs,
	RunE: func(command *cobra.Command, _ []string) error {
		update, changing, parseError := parseSettingsFlags()
		if parseError != nil {
			return withExitCode(exitUsage, parseError)
		}
		settings, getError := apiClient.GetAISettings(command.Context())
		if getError != nil {
			return getError
		}
		if changing {
			if update.PreApproved != nil {
				merged := mergePreApprovals(settings.PreApproved, *update.PreApproved)
				update.PreApproved = &merged
			}
			saved, saveError := apiClient.UpdateAISettings(command.Context(), update)
			if saveError != nil {
				return saveError
			}
			settings = saved
		}
		if rendered, renderError := renderJSON(command, settings); rendered || renderError != nil {
			return renderError
		}
		fmt.Fprint(command.OutOrStdout(), renderAISettings(settings))
		return nil
	},
}

func init() {
	settingsAICmd.Flags().StringVar(&settingsApproversFlag, "approvers", "", "who may approve: admins or members")
	settingsAICmd.Flags().StringArrayVar(&settingsPreApproveFlags, "pre-approve", nil, "let a category run unasked, up to N a month: category=N (repeatable; 0 asks again)")
	settingsAICmd.Flags().StringVar(&settingsQuietHoursFlag, "quiet-hours", "", "keep scheduled work quiet: HH:MM-HH:MM[@Zone]")
	settingsAICmd.Flags().BoolVar(&settingsClearQuietHoursFlag, "clear-quiet-hours", false, "lift the quiet hours")
	settingsCmd.AddCommand(settingsAICmd)
	rootCmd.AddCommand(settingsCmd)
}

var quietHoursPattern = regexp.MustCompile(`^(\d{2}:\d{2})-(\d{2}:\d{2})(?:@(\S+))?$`)

// parseSettingsFlags turns the flags into an update, and says whether
// anything is being changed at all.
func parseSettingsFlags() (client.AISettingsUpdate, bool, error) {
	var update client.AISettingsUpdate
	changing := false
	if settingsApproversFlag != "" {
		if settingsApproversFlag != "admins" && settingsApproversFlag != "members" {
			return update, false, fmt.Errorf("--approvers takes admins or members, not %q", settingsApproversFlag)
		}
		approvers := settingsApproversFlag
		update.Approvers = &approvers
		changing = true
	}
	if len(settingsPreApproveFlags) > 0 {
		preApproved := map[string]client.PreApproval{}
		for _, assignment := range settingsPreApproveFlags {
			category, count, found := strings.Cut(assignment, "=")
			category = strings.TrimSpace(category)
			perMonth, parseError := strconv.Atoi(strings.TrimSpace(count))
			if !found || category == "" || parseError != nil || perMonth < 0 {
				return update, false, fmt.Errorf("--pre-approve takes category=N, got %q", assignment)
			}
			preApproved[category] = client.PreApproval{PerMonth: perMonth}
		}
		update.PreApproved = &preApproved
		changing = true
	}
	if settingsQuietHoursFlag != "" && settingsClearQuietHoursFlag {
		return update, false, fmt.Errorf("say --quiet-hours or --clear-quiet-hours, not both")
	}
	if settingsQuietHoursFlag != "" {
		matches := quietHoursPattern.FindStringSubmatch(strings.TrimSpace(settingsQuietHoursFlag))
		if matches == nil {
			return update, false, fmt.Errorf("--quiet-hours takes HH:MM-HH:MM[@Zone], got %q", settingsQuietHoursFlag)
		}
		timezone := matches[3]
		if timezone == "" {
			timezone = "UTC"
		}
		update.QuietHours = &client.QuietHours{Start: matches[1], End: matches[2], Timezone: timezone}
		changing = true
	}
	if settingsClearQuietHoursFlag {
		update.ClearQuietHours = true
		changing = true
	}
	return update, changing, nil
}

// mergePreApprovals lays the changes over what is set, since the API
// replaces the whole map: a zero removes the category.
func mergePreApprovals(current map[string]client.PreApproval, changes map[string]client.PreApproval) map[string]client.PreApproval {
	merged := map[string]client.PreApproval{}
	for category, preApproval := range current {
		merged[category] = preApproval
	}
	for category, preApproval := range changes {
		if preApproval.PerMonth == 0 {
			delete(merged, category)
			continue
		}
		merged[category] = preApproval
	}
	return merged
}

// renderAISettings says how Tael is set, one line each.
func renderAISettings(settings *client.AISettings) string {
	var builder strings.Builder
	fmt.Fprintf(&builder, "Tael on this workspace (plan: %s)\n", valueOrDash(settings.Plan))
	paused := "no"
	if settings.Paused {
		paused = "yes — nothing starts until `tael resume`"
		if settings.PausedBy != nil && *settings.PausedBy != "" {
			paused += " (paused by " + *settings.PausedBy + ")"
		}
	}
	fmt.Fprintf(&builder, "Paused:       %s\n", paused)
	approvers := "owners and admins"
	if settings.Approvers == "members" {
		approvers = "every member"
	}
	fmt.Fprintf(&builder, "Approvers:    %s\n", approvers)
	categories := make([]string, 0, len(settings.PreApproved))
	for category, preApproval := range settings.PreApproved {
		if preApproval.PerMonth > 0 {
			categories = append(categories, category)
		}
	}
	sort.Strings(categories)
	if len(categories) == 0 {
		builder.WriteString("Pre-approved: nothing — Tael asks every time\n")
	} else {
		allowances := make([]string, 0, len(categories))
		for _, category := range categories {
			allowances = append(allowances, fmt.Sprintf("%s up to %d a month", strings.ReplaceAll(category, "_", " "), settings.PreApproved[category].PerMonth))
		}
		fmt.Fprintf(&builder, "Pre-approved: %s\n", strings.Join(allowances, "; "))
	}
	if settings.QuietHours != nil && settings.QuietHours.Start != "" {
		fmt.Fprintf(&builder, "Quiet hours:  %s–%s %s\n", settings.QuietHours.Start, settings.QuietHours.End, settings.QuietHours.Timezone)
	} else {
		builder.WriteString("Quiet hours:  none\n")
	}
	if settings.Budget > 0 {
		fmt.Fprintf(&builder, "This month:   %d of %d actions used\n", settings.ActionsThisMonth, settings.Budget)
	}
	if settings.AllowanceExhaustedUntil != nil && *settings.AllowanceExhaustedUntil != "" {
		fmt.Fprintf(&builder, "The month's allowance is used up until %s; Tael asks for everything until then.\n", formatTimestamp(*settings.AllowanceExhaustedUntil))
	}
	return builder.String()
}
