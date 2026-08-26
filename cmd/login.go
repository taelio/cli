package cmd

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"tael.io/cli/internal/client"
)

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Authenticate with the tael platform",
	Long: `Authenticate with the tael platform using your browser.

The CLI generates a PKCE key pair, opens your browser to the tael login
page, and waits for you to approve the sign-in. The code verifier never
leaves this machine and no local network port is opened.

Your credentials are saved to ~/.tael.yaml.`,
	RunE: func(command *cobra.Command, _ []string) error {
		if loginError := runLogin(command); loginError != nil {
			return fmt.Errorf("login failed: %w", loginError)
		}
		return nil
	},
}

func init() {
	setRequiresAuth(loginCmd, false)
	rootCmd.AddCommand(loginCmd)
}

func runLogin(command *cobra.Command) error {
	out := command.OutOrStdout()
	configuration := resolveConfiguration(command)
	loginClient := client.New("", configuration.BaseURL, version)

	// The PKCE verifier stays on this machine; only its S256 challenge is
	// sent on init. Presenting the verifier on poll is what authorises
	// collecting the token the browser login parked on the ticket.
	codeVerifier, verifierError := generateCodeVerifier()
	if verifierError != nil {
		return fmt.Errorf("generate code verifier: %w", verifierError)
	}
	codeChallenge := generateCodeChallenge(codeVerifier)

	machineName, hostnameError := os.Hostname()
	if hostnameError != nil {
		machineName = ""
	}

	initResponse, initError := loginClient.LoginInit(command.Context(), client.LoginInitRequest{
		CodeChallenge: codeChallenge,
		MachineName:   machineName,
	})
	if initError != nil {
		return initError
	}

	fmt.Fprintln(out, "Opening browser for authentication...")
	fmt.Fprintln(out)
	fmt.Fprintln(out, "If the browser doesn't open, visit this URL:")
	fmt.Fprintln(out)
	fmt.Fprintf(out, "  %s\n", initResponse.AuthURL)
	fmt.Fprintln(out)
	if browserError := openBrowser(initResponse.AuthURL); browserError != nil {
		fmt.Fprintln(out, "Could not open the browser automatically; open the URL above manually.")
	}
	fmt.Fprintln(out, "Waiting for you to approve the sign-in in your browser... (Ctrl+C to cancel)")

	pollResponse, pollError := pollForToken(command, loginClient, initResponse.Ticket, codeVerifier)
	if pollError != nil {
		return pollError
	}

	if writeError := writeConfigFile(savedConfig{
		Token:     pollResponse.Token,
		BaseURL:   configuration.BaseURL,
		Workspace: pollResponse.WorkspaceSlug,
	}); writeError != nil {
		return writeError
	}

	fmt.Fprintln(out)
	fmt.Fprintln(out, "Login successful.")
	fmt.Fprintf(out, "  Workspace:   %s\n", pollResponse.WorkspaceSlug)
	fmt.Fprintf(out, "  Credentials: %s\n", configFilePath())
	fmt.Fprintln(out)
	fmt.Fprintln(out, "Try `tael apps` to list your apps.")
	return nil
}

// pollForToken polls login/poll every two seconds until the browser login
// releases the token, the platform reports a terminal error, or ten
// minutes pass.
func pollForToken(command *cobra.Command, loginClient *client.Client, ticket string, codeVerifier string) (*client.LoginPollResponse, error) {
	deadline := time.After(10 * time.Minute)
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	pollRequest := client.LoginPollRequest{Ticket: ticket, CodeVerifier: codeVerifier}
	for {
		select {
		case <-command.Context().Done():
			return nil, command.Context().Err()
		case <-deadline:
			return nil, fmt.Errorf("login timed out after 10 minutes")
		case <-ticker.C:
			pollResponse, pollError := loginClient.LoginPoll(command.Context(), pollRequest)
			if pollError != nil {
				var apiError *client.APIError
				if errors.As(pollError, &apiError) && apiError.StatusCode >= 500 {
					continue // server hiccup — keep polling
				}
				return nil, pollError
			}
			if pollResponse.Token != "" {
				return pollResponse, nil
			}
		}
	}
}

func generateCodeVerifier() (string, error) {
	randomBytes := make([]byte, 32)
	if _, readError := rand.Read(randomBytes); readError != nil {
		return "", readError
	}
	return base64.RawURLEncoding.EncodeToString(randomBytes), nil
}

func generateCodeChallenge(codeVerifier string) string {
	digest := sha256.Sum256([]byte(codeVerifier))
	return base64.RawURLEncoding.EncodeToString(digest[:])
}
