package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

type outputFormat string

const (
	outputText outputFormat = "text"
	outputJSON outputFormat = "json"
)

// parseOutputFormat validates the global -o/--output value.
func parseOutputFormat(raw string) (outputFormat, error) {
	switch raw {
	case "", string(outputText):
		return outputText, nil
	case string(outputJSON):
		return outputJSON, nil
	default:
		return outputText, fmt.Errorf("invalid output format %q: must be text or json", raw)
	}
}

// renderJSON writes value as indented JSON when -o json was requested. It
// reports true when JSON was written, so the caller skips its text
// rendering.
func renderJSON(command *cobra.Command, value any) (bool, error) {
	format, formatError := parseOutputFormat(outputFlag)
	if formatError != nil {
		return false, withExitCode(exitUsage, formatError)
	}
	if format != outputJSON {
		return false, nil
	}
	encoder := json.NewEncoder(command.OutOrStdout())
	encoder.SetIndent("", "  ")
	return true, encoder.Encode(value)
}
