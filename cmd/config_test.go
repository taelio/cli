package cmd

import (
	"os"
	"path/filepath"
	"testing"
)

// TestConfigFileRoundTrip covers the save path used by `tael login` and the
// clear path used by `tael logout`, including the 0600 permission
// discipline on the credential file.
func TestConfigFileRoundTrip(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "tael.yaml")
	t.Setenv("TAEL_CONFIG", configPath)

	written := savedConfig{
		Token:     "tael_token_123",
		BaseURL:   "https://api.tael.test",
		Workspace: "acme",
	}
	if writeError := writeConfigFile(written); writeError != nil {
		t.Fatalf("writeConfigFile: %v", writeError)
	}

	info, statError := os.Stat(configPath)
	if statError != nil {
		t.Fatalf("stat config: %v", statError)
	}
	if permissions := info.Mode().Perm(); permissions != 0o600 {
		t.Errorf("config permissions = %#o, want 0600", permissions)
	}

	read := readConfigFile()
	if read != written {
		t.Errorf("readConfigFile = %+v, want %+v", read, written)
	}

	read.Token = ""
	if clearError := writeConfigFile(read); clearError != nil {
		t.Fatalf("clear token: %v", clearError)
	}
	if cleared := readConfigFile(); cleared.Token != "" || cleared.Workspace != "acme" {
		t.Errorf("after clearing token got %+v, want empty token with workspace kept", cleared)
	}
}
