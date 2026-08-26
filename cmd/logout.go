package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var logoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Remove the saved credentials",
	Long:  `Clear the saved API token from ~/.tael.yaml.`,
	RunE: func(command *cobra.Command, _ []string) error {
		out := command.OutOrStdout()
		saved := readConfigFile()
		if saved.Token == "" {
			fmt.Fprintln(out, "No credentials found.")
			return nil
		}
		saved.Token = ""
		if writeError := writeConfigFile(saved); writeError != nil {
			return fmt.Errorf("clearing credentials: %w", writeError)
		}
		fmt.Fprintf(out, "Logged out. Token removed from %s\n", configFilePath())
		return nil
	},
}

func init() {
	setRequiresAuth(logoutCmd, false)
	rootCmd.AddCommand(logoutCmd)
}
