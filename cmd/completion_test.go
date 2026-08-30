package cmd

import (
	"context"
	"io"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
)

// TestCompletionNeedsNoLogin runs the shell completion generator the way
// Homebrew does at install time: no token anywhere, no config file yet.
// Cobra binds the generator's writer on the first execution in a process,
// so the script itself is not captured here; the refusal is what matters.
func TestCompletionNeedsNoLogin(t *testing.T) {
	t.Setenv("TAEL_CONFIG", filepath.Join(t.TempDir(), "tael.yaml"))
	t.Setenv(envAPIToken, "")
	t.Setenv(envBaseURL, "")
	t.Setenv(envWorkspace, "")
	resetFlags(rootCmd)

	rootCmd.SetOut(io.Discard)
	rootCmd.SetErr(io.Discard)
	rootCmd.SetArgs([]string{"completion", "zsh"})
	t.Cleanup(func() {
		resetFlags(rootCmd)
		rootCmd.SetOut(nil)
		rootCmd.SetErr(nil)
		rootCmd.SetArgs(nil)
	})
	if _, executeError := rootCmd.ExecuteContextC(context.Background()); executeError != nil {
		t.Fatalf("tael completion zsh must not need a login, got: %v", executeError)
	}
}

// TestCommandRequiresAuthExemptsOnlyTheRootCompletion pins the rule: the
// `completion` command directly under the root and its shells are exempt;
// a command that merely happens to be called completion elsewhere is not.
func TestCommandRequiresAuthExemptsOnlyTheRootCompletion(t *testing.T) {
	root := &cobra.Command{Use: "tael"}
	completion := &cobra.Command{Use: "completion"}
	zsh := &cobra.Command{Use: "zsh"}
	root.AddCommand(completion)
	completion.AddCommand(zsh)

	apps := &cobra.Command{Use: "apps"}
	nested := &cobra.Command{Use: "completion"}
	root.AddCommand(apps)
	apps.AddCommand(nested)

	if commandRequiresAuth(completion) || commandRequiresAuth(zsh) {
		t.Fatal("completion and its shells must not require a login")
	}
	if !commandRequiresAuth(apps) || !commandRequiresAuth(nested) {
		t.Fatal("every other command still requires a login")
	}
}
