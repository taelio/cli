// Package cmd wires the tael command tree. Configuration is resolved per
// setting with flag > environment > config file > default precedence.
package cmd

import (
	"errors"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"tael.io/cli/internal/client"
)

const (
	envAPIToken    = "TAEL_API_TOKEN"
	envBaseURL     = "TAEL_BASE_URL"
	envWorkspace   = "TAEL_WORKSPACE"
	defaultBaseURL = "https://api.tael.io"

	// annotationRequiresAuth controls whether the persistent pre-run hook
	// requires an API token before invoking a command's RunE. Commands that
	// do not call the tael API (login, logout, init, version, help) set it
	// to "false" so users can invoke them without a token.
	annotationRequiresAuth = "tael.requires_auth"
)

var (
	tokenFlag     string
	baseURLFlag   string
	workspaceFlag string
	outputFlag    string

	apiClient *client.Client
)

var rootCmd = &cobra.Command{
	Use:   "tael",
	Short: "CLI for the tael AI DevOps platform",
	Long: `tael manages your apps on the tael platform: connect a repository,
trigger deploys, follow logs, and check what is live.`,
	SilenceUsage:      true,
	PersistentPreRunE: persistentPreRunE,
}

// Execute runs the root command and exits the process with the CLI's
// documented exit codes (0 ok, 1 failure, 2 usage, 3 auth).
func Execute() {
	rootCmd.Version = version
	if _, executeError := rootCmd.ExecuteC(); executeError != nil {
		os.Exit(exitCodeFor(executeError))
	}
}

func init() {
	persistentFlags := rootCmd.PersistentFlags()
	persistentFlags.StringVar(&tokenFlag, "token", "", "API token (or set "+envAPIToken+")")
	persistentFlags.StringVar(&baseURLFlag, "base-url", "", "Base URL for the tael API (or set "+envBaseURL+")")
	persistentFlags.StringVar(&workspaceFlag, "workspace", "", "Workspace slug to run against (or set "+envWorkspace+")")
	persistentFlags.StringVarP(&outputFlag, "output", "o", "text", "Output format: text or json")

	rootCmd.SetFlagErrorFunc(func(_ *cobra.Command, flagError error) error {
		return withExitCode(exitUsage, flagError)
	})
}

// persistentPreRunE validates global flags and resolves credentials for any
// command that requires authentication.
func persistentPreRunE(command *cobra.Command, _ []string) error {
	if _, formatError := parseOutputFormat(outputFlag); formatError != nil {
		return withExitCode(exitUsage, formatError)
	}
	if !commandRequiresAuth(command) {
		return nil
	}

	configuration := resolveConfiguration(command)
	if configuration.Token == "" {
		return withExitCode(exitAuth, errors.New(
			"not logged in: run `tael login`, or provide a token via --token or "+envAPIToken))
	}
	apiClient = client.New(configuration.Token, configuration.BaseURL, version)
	apiClient.WorkspaceID = configuration.WorkspaceID
	return nil
}

// configuration is the fully resolved set of global settings for a command
// invocation.
type configuration struct {
	Token     string
	BaseURL   string
	Workspace string
	// WorkspaceID is the workspace chosen with `tael workspace use`, from
	// the config file only; the flag and environment carry the slug.
	WorkspaceID string
}

// resolveConfiguration merges the three configuration sources for every
// setting, with flag > environment > config file > default precedence.
func resolveConfiguration(command *cobra.Command) configuration {
	saved := readConfigFile()
	flags := command.Root().PersistentFlags()
	return configuration{
		Token:       resolveSetting(flagStringValue(flags, "token"), os.Getenv(envAPIToken), saved.Token, ""),
		BaseURL:     resolveSetting(flagStringValue(flags, "base-url"), os.Getenv(envBaseURL), saved.BaseURL, defaultBaseURL),
		Workspace:   resolveSetting(flagStringValue(flags, "workspace"), os.Getenv(envWorkspace), saved.Workspace, ""),
		WorkspaceID: saved.WorkspaceID,
	}
}

// resolveSetting picks the first non-empty value in precedence order:
// flag, then environment, then config file, then the built-in default.
func resolveSetting(flagValue string, environmentValue string, fileValue string, defaultValue string) string {
	switch {
	case flagValue != "":
		return flagValue
	case environmentValue != "":
		return environmentValue
	case fileValue != "":
		return fileValue
	default:
		return defaultValue
	}
}

// flagStringValue returns a persistent flag's value only when the user set
// it on the command line, so an unset flag never shadows env or file values.
func flagStringValue(flags *pflag.FlagSet, name string) string {
	flag := flags.Lookup(name)
	if flag == nil || !flag.Changed {
		return ""
	}
	return flag.Value.String()
}

// setRequiresAuth attaches the auth requirement annotation to a command.
func setRequiresAuth(command *cobra.Command, required bool) {
	if command.Annotations == nil {
		command.Annotations = map[string]string{}
	}
	if required {
		command.Annotations[annotationRequiresAuth] = "true"
	} else {
		command.Annotations[annotationRequiresAuth] = "false"
	}
}

// commandRequiresAuth resolves the auth requirement by walking up the
// command tree, defaulting to true so any new command is fail-safe. Cobra's
// built-in help and completion helpers never call the API.
func commandRequiresAuth(command *cobra.Command) bool {
	if command == nil {
		return false
	}
	switch command.Name() {
	case "help", cobra.ShellCompRequestCmd, cobra.ShellCompNoDescRequestCmd:
		return false
	}
	for current := command; current != nil; current = current.Parent() {
		if current.Annotations == nil {
			continue
		}
		if value, ok := current.Annotations[annotationRequiresAuth]; ok {
			return value != "false"
		}
	}
	return true
}
