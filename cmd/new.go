package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"tael.io/cli/internal/client"
)

var (
	newRepoFlag     string
	newBranchFlag   string
	newNameFlag     string
	newDatabaseFlag bool
	newGoLiveFlag   bool
	newNoFollowFlag bool
)

// setupPollInterval is how often the setup is read while following it,
// on top of the event stream; a test shortens it.
var setupPollInterval = 5 * time.Second

var newCmd = &cobra.Command{
	Use:   "new --repo owner/name [--branch main] [--name app]",
	Short: "Put a repository live: Tael reads it, sets it up and opens the setup pull request",
	Long: `Make an app from one of the repositories Tael can see (list them with
` + "`tael repos`" + `). Tael reads the repository first and says what it found, then
sets the app up and follows the setup until the pull request is ready.
Merge it with ` + "`tael go-live`" + `, or pass --go-live to merge it as soon as it is ready.
--database adds a Tael Managed Postgres for the app right behind it.`,
	Args: cobra.NoArgs,
	RunE: func(command *cobra.Command, _ []string) error {
		out := command.OutOrStdout()
		repository, findError := findRepository(command.Context(), newRepoFlag)
		if findError != nil {
			return findError
		}
		branch := firstNonEmpty(newBranchFlag, repository.DefaultBranch)

		fmt.Fprintf(out, "Reading %s…\n", repository.FullName)
		analysis, analyseError := apiClient.AnalyseRepository(command.Context(), client.AnalyseRepositoryRequest{
			RepoFullName: repository.FullName, DefaultBranch: branch, InstallationID: repository.InstallationID,
		})
		switch {
		case analyseError == nil:
			fmt.Fprint(out, renderAnalysis(analysis))
		case isSkippableAnalysisError(analyseError):
			fmt.Fprintln(out, "Tael could not read it first; carrying on.")
		default:
			return analyseError
		}

		created, createError := apiClient.CreateApp(command.Context(), client.CreateAppRequest{
			Name: newNameFlag, RepoFullName: repository.FullName, DefaultBranch: branch, InstallationID: repository.InstallationID,
		})
		if createError != nil {
			return createError
		}
		appName := firstNonEmpty(newNameFlag, repositoryName(repository.FullName))
		if rendered, renderError := renderJSON(command, map[string]any{"app": created, "analysis": analysis}); rendered || renderError != nil {
			return renderError
		}
		fmt.Fprintf(out, "Setting up %s. Tael is reading the repository and will open a setup pull request.\n", appName)

		if newDatabaseFlag {
			_, installError := apiClient.InstallSolution(command.Context(), client.InstallSolutionRequest{SolutionKey: "postgres", Preset: "small", AppID: created.ID})
			if installError != nil {
				fmt.Fprintf(out, "The app is being set up, but not its database: %s\n", refusalSentence(installError))
			} else {
				fmt.Fprintln(out, "Adding a Tael Managed Postgres for it; its connection arrives as DATABASE_URL.")
			}
		}
		if newNoFollowFlag {
			fmt.Fprintf(out, "Follow it with `tael setup %s`.\n", appName)
			return nil
		}

		fmt.Fprintln(out, "Following the setup (Ctrl+C to stop; `tael setup` later).")
		setup, followError := followSetup(command.Context(), out, created.ID)
		if followError != nil {
			return followError
		}
		if setup == nil {
			return nil
		}
		if setup.Status == "failed" {
			reason := strings.TrimSpace(setup.ErrorMessage)
			if reason == "" {
				reason = "Something in the setup did not work. The repository was not touched."
			}
			fmt.Fprintf(out, "Stopped: %s\n→ tael retry %s\n", reason, appName)
			return nil
		}
		fmt.Fprintf(out, "The setup pull request is ready: %s\n", setup.PullRequestURL)
		if !newGoLiveFlag {
			fmt.Fprintf(out, "→ tael go-live %s\n", appName)
			return nil
		}
		goLive, goLiveError := apiClient.GoLive(command.Context(), created.ID)
		if goLiveError != nil {
			return goLiveError
		}
		fmt.Fprintf(out, "Merged it; %s is going live. Follow it with `tael logs %s -f`.\n", appName, appName)
		_ = goLive
		return nil
	},
}

func init() {
	newCmd.Flags().StringVar(&newRepoFlag, "repo", "", "the repository, as owner/name")
	newCmd.Flags().StringVar(&newBranchFlag, "branch", "", "the branch to deploy (the repository's default when unset)")
	newCmd.Flags().StringVar(&newNameFlag, "name", "", "the app's name (the repository's name when unset)")
	newCmd.Flags().BoolVar(&newDatabaseFlag, "database", false, "add a Tael Managed Postgres for the app")
	newCmd.Flags().BoolVar(&newGoLiveFlag, "go-live", false, "merge the setup pull request as soon as it is ready")
	newCmd.Flags().BoolVar(&newNoFollowFlag, "no-follow", false, "return once the setup has started instead of following it")
	_ = newCmd.MarkFlagRequired("repo")
	rootCmd.AddCommand(newCmd)
}

func repositoryName(fullName string) string {
	_, name, found := strings.Cut(fullName, "/")
	if !found {
		return fullName
	}
	return name
}

// findRepository finds the repository the person named among those Tael
// can see. An unknown name lists what there is.
func findRepository(requestContext context.Context, word string) (*client.Repository, error) {
	trimmed := strings.TrimSpace(word)
	if strings.Count(trimmed, "/") != 1 {
		return nil, withExitCode(exitUsage, fmt.Errorf("say the repository as owner/name: --repo acme/web"))
	}
	listResponse, listError := apiClient.ListRepositories(requestContext)
	if listError != nil {
		return nil, listError
	}
	for index := range listResponse.Repos {
		if strings.EqualFold(listResponse.Repos[index].FullName, trimmed) {
			return &listResponse.Repos[index], nil
		}
	}
	if len(listResponse.Repos) == 0 {
		return nil, withExitCode(exitUsage, fmt.Errorf("Tael cannot see any repositories yet; install the Tael GitHub App from the web app first"))
	}
	names := make([]string, 0, len(listResponse.Repos))
	for _, repo := range listResponse.Repos {
		names = append(names, repo.FullName)
	}
	return nil, withExitCode(exitUsage, fmt.Errorf("Tael cannot see %q; it can see: %s", trimmed, strings.Join(names, ", ")))
}

// isSkippableAnalysisError is true when the reading is off or did not
// work — the app still gets made, as the web app does.
func isSkippableAnalysisError(analyseError error) bool {
	var apiError *client.APIError
	if !errors.As(analyseError, &apiError) {
		return false
	}
	return apiError.StatusCode == http.StatusNotImplemented || apiError.StatusCode == http.StatusBadGateway
}

// refusalSentence is the API's sentence for a refusal, without the status.
func refusalSentence(failure error) string {
	var apiError *client.APIError
	if errors.As(failure, &apiError) && apiError.Detail != "" {
		return apiError.Detail
	}
	return failure.Error()
}

// renderAnalysis says what Tael read the repository to be, in the words
// onboarding uses.
func renderAnalysis(analysis *client.RepoAnalysis) string {
	var builder strings.Builder
	framework := analysis.Framework
	if framework == "" {
		framework = "a web app"
	}
	fmt.Fprintf(&builder, "This looks like %s.", framework)
	if summary := strings.TrimSpace(analysis.Summary); summary != "" {
		fmt.Fprintf(&builder, " %s", summary)
	}
	builder.WriteString("\n")
	reason := strings.TrimSpace(analysis.DatabaseReason)
	if analysis.NeedsDatabase {
		if reason == "" {
			reason = "the code looks like it expects one"
		}
		fmt.Fprintf(&builder, "It wants a database: %s Add one with --database.\n", withFullStop(reason))
	} else if reason != "" {
		fmt.Fprintf(&builder, "It does not need a database: %s\n", withFullStop(reason))
	}
	if len(analysis.RequiredEnvironment) > 0 {
		fmt.Fprintf(&builder, "It reads these variables: %s\n", strings.Join(analysis.RequiredEnvironment, ", "))
	}
	for _, concern := range analysis.Concerns {
		if trimmed := strings.TrimSpace(concern); trimmed != "" {
			fmt.Fprintf(&builder, "Worth knowing: %s\n", withFullStop(trimmed))
		}
	}
	return builder.String()
}

func withFullStop(sentence string) string {
	if strings.HasSuffix(sentence, ".") || strings.HasSuffix(sentence, "!") || strings.HasSuffix(sentence, "?") {
		return sentence
	}
	return sentence + "."
}

// setupStepNarration is what each step of an app's setup reads as while
// it runs. Keyed by the queue's step name, which is never printed.
var setupStepNarration = map[string]string{
	"app_onboard":            "Getting your app ready to go live.",
	"app_create_application": "Registering your app.",
	"app_watch_generation":   "Reading your repo and working out what it needs.",
	"app_watch_installation": "Rolling your app out.",
	"app_deploy_application": "Deploying your app.",
}

// setupEventPayload is the part of a setup event the CLI reads.
type setupEventPayload struct {
	AppID string `json:"app_id"`
	Name  string `json:"name"`
}

// narrateSetupEvent turns one stream event into a line for the person
// following a setup. It answers refresh=true when the event is about this
// app's setup, so the caller reads where it stands.
func narrateSetupEvent(event client.Event, appID string) (line string, refresh bool) {
	var payload setupEventPayload
	if len(event.Payload) > 0 {
		if unmarshalError := json.Unmarshal(event.Payload, &payload); unmarshalError != nil {
			return "", false
		}
	}
	switch event.EventType {
	case "app_analysis_progress":
		return "", payload.AppID == appID
	case "step_started":
		return setupStepNarration[payload.Name], false
	case "step_failed":
		if _, ours := setupStepNarration[payload.Name]; ours {
			return "That step did not work; checking where the setup stands.", true
		}
	}
	return "", false
}

// followSetup follows an app's setup — the event stream for immediacy, a
// read of the setup every few seconds for certainty — printing each step
// as Tael takes it, until the pull request is ready or the setup stops.
// A cancelled context ends it with a nil setup.
func followSetup(requestContext context.Context, out io.Writer, appID string) (*client.AppSetup, error) {
	streamContext, stopStream := context.WithCancel(requestContext)
	defer stopStream()
	events := make(chan client.Event, 16)
	go func() {
		defer close(events)
		_ = apiClient.FollowEvents(streamContext, func(event client.Event) bool {
			select {
			case events <- event:
				return true
			case <-streamContext.Done():
				return false
			}
		})
	}()

	seen := map[string]string{}
	refresh := func() (*client.AppSetup, bool) {
		setup, setupError := apiClient.GetAppSetup(requestContext, appID)
		if setupError != nil {
			// Early on the runtime has nothing to say yet; the next read will.
			return nil, false
		}
		for _, entry := range setup.CreationProgress {
			if seen[entry.Key] == entry.Status {
				continue
			}
			seen[entry.Key] = entry.Status
			if message := strings.TrimSpace(entry.Message); message != "" {
				fmt.Fprintf(out, "  %s %s\n", progressMark(entry.Status), message)
			}
		}
		return setup, setup.PullRequestURL != "" || setup.Status == "failed"
	}
	if setup, finished := refresh(); finished {
		return setup, nil
	}
	ticker := time.NewTicker(setupPollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-requestContext.Done():
			return nil, nil
		case event, open := <-events:
			if !open {
				// The stream ended; the reads carry on alone.
				events = nil
				continue
			}
			line, ours := narrateSetupEvent(event, appID)
			if line != "" {
				fmt.Fprintln(out, line)
			}
			if ours {
				if setup, finished := refresh(); finished {
					return setup, nil
				}
			}
		case <-ticker.C:
			if setup, finished := refresh(); finished {
				return setup, nil
			}
		}
	}
}
