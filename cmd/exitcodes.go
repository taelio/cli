package cmd

import (
	"errors"
	"net/http"
	"strings"

	"tael.io/cli/internal/client"
)

// Exit codes are part of the CLI's scripting contract: scripts can react to
// a failure class (fix the invocation, re-authenticate) without parsing
// error text.
const (
	exitOK    = 0
	exitError = 1 // API or runtime failure
	exitUsage = 2 // bad flags, arguments, or subcommand
	exitAuth  = 3 // missing, expired, or rejected credentials
)

// codedError attaches an exit code to an error without changing its message.
type codedError struct {
	code    int
	wrapped error
}

func (coded *codedError) Error() string { return coded.wrapped.Error() }
func (coded *codedError) Unwrap() error { return coded.wrapped }

// withExitCode wraps wrappedError so Execute exits with code. A nil error
// stays nil, so call sites can wrap unconditionally.
func withExitCode(code int, wrappedError error) error {
	if wrappedError == nil {
		return nil
	}
	return &codedError{code: code, wrapped: wrappedError}
}

// exitCodeFor classifies an error returned by ExecuteC into an exit code.
// Explicit withExitCode wrapping wins; otherwise auth API responses map to
// exitAuth, Cobra's fixed usage-error shapes map to exitUsage, and anything
// unclassified is a generic failure.
func exitCodeFor(commandError error) int {
	if commandError == nil {
		return exitOK
	}
	var coded *codedError
	if errors.As(commandError, &coded) {
		return coded.code
	}
	if errors.Is(commandError, client.ErrUnauthorized) {
		return exitAuth
	}
	var apiError *client.APIError
	if errors.As(commandError, &apiError) {
		switch apiError.StatusCode {
		case http.StatusUnauthorized, http.StatusForbidden:
			return exitAuth
		}
	}
	// Cobra reports unknown subcommands and required-flag violations as
	// plain errors that bypass FlagErrorFunc, so match their fixed shapes.
	message := commandError.Error()
	if strings.HasPrefix(message, "unknown command ") ||
		strings.HasPrefix(message, "required flag(s) ") ||
		strings.HasPrefix(message, "accepts ") {
		return exitUsage
	}
	return exitError
}
