package cmd

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveSettingPrecedence(t *testing.T) {
	testCases := []struct {
		name             string
		flagValue        string
		environmentValue string
		fileValue        string
		defaultValue     string
		expected         string
	}{
		{name: "flag beats env and file", flagValue: "from-flag", environmentValue: "from-env", fileValue: "from-file", defaultValue: "fallback", expected: "from-flag"},
		{name: "env beats file", flagValue: "", environmentValue: "from-env", fileValue: "from-file", defaultValue: "fallback", expected: "from-env"},
		{name: "file beats default", flagValue: "", environmentValue: "", fileValue: "from-file", defaultValue: "fallback", expected: "from-file"},
		{name: "default when nothing set", flagValue: "", environmentValue: "", fileValue: "", defaultValue: "fallback", expected: "fallback"},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			resolved := resolveSetting(testCase.flagValue, testCase.environmentValue, testCase.fileValue, testCase.defaultValue)
			if resolved != testCase.expected {
				t.Fatalf("resolveSetting = %q, want %q", resolved, testCase.expected)
			}
		})
	}
}

// TestResolveConfigurationPrecedence exercises the full flag > env > file
// chain through resolveConfiguration, using a real config file and the real
// root command flags.
func TestResolveConfigurationPrecedence(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "tael.yaml")
	fileContent := "token: file-token\nbase_url: https://file.tael.test\nworkspace: file-workspace\n"
	if writeError := os.WriteFile(configPath, []byte(fileContent), 0o600); writeError != nil {
		t.Fatalf("write config fixture: %v", writeError)
	}
	t.Setenv("TAEL_CONFIG", configPath)
	t.Setenv(envAPIToken, "env-token")
	t.Setenv(envBaseURL, "")
	t.Setenv(envWorkspace, "")

	tokenFlagDefinition := rootCmd.PersistentFlags().Lookup("token")
	if setError := rootCmd.PersistentFlags().Set("token", "flag-token"); setError != nil {
		t.Fatalf("set token flag: %v", setError)
	}
	t.Cleanup(func() {
		_ = tokenFlagDefinition.Value.Set("")
		tokenFlagDefinition.Changed = false
	})

	resolved := resolveConfiguration(rootCmd)

	if resolved.Token != "flag-token" {
		t.Errorf("Token = %q, want flag value to beat env and file", resolved.Token)
	}
	if resolved.BaseURL != "https://file.tael.test" {
		t.Errorf("BaseURL = %q, want file value to beat the default", resolved.BaseURL)
	}
	if resolved.Workspace != "file-workspace" {
		t.Errorf("Workspace = %q, want file value", resolved.Workspace)
	}

	t.Setenv(envAPIToken, "")
	resolvedWithoutEnv := resolveConfiguration(rootCmd)
	if resolvedWithoutEnv.Token != "flag-token" {
		t.Errorf("Token = %q, want flag value with no env set", resolvedWithoutEnv.Token)
	}

	tokenFlagDefinition.Changed = false
	_ = tokenFlagDefinition.Value.Set("")
	t.Setenv(envAPIToken, "env-token")
	resolvedFromEnv := resolveConfiguration(rootCmd)
	if resolvedFromEnv.Token != "env-token" {
		t.Errorf("Token = %q, want env value to beat file", resolvedFromEnv.Token)
	}

	t.Setenv(envAPIToken, "")
	resolvedFromFile := resolveConfiguration(rootCmd)
	if resolvedFromFile.Token != "file-token" {
		t.Errorf("Token = %q, want file value when flag and env are unset", resolvedFromFile.Token)
	}
	if resolvedFromFile.BaseURL != "https://file.tael.test" {
		t.Errorf("BaseURL = %q, want file value", resolvedFromFile.BaseURL)
	}

	if removeError := os.Remove(configPath); removeError != nil {
		t.Fatalf("remove config fixture: %v", removeError)
	}
	resolvedDefaults := resolveConfiguration(rootCmd)
	if resolvedDefaults.BaseURL != defaultBaseURL {
		t.Errorf("BaseURL = %q, want default %q with no sources", resolvedDefaults.BaseURL, defaultBaseURL)
	}
	if resolvedDefaults.Token != "" {
		t.Errorf("Token = %q, want empty with no sources", resolvedDefaults.Token)
	}
}
