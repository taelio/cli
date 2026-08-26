package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

// savedConfig mirrors the keys of ~/.tael.yaml.
type savedConfig struct {
	Token     string
	BaseURL   string
	Workspace string
}

// configFilePath returns the config file location, honouring the
// TAEL_CONFIG override used by tests.
func configFilePath() string {
	if override := os.Getenv("TAEL_CONFIG"); override != "" {
		return override
	}
	home, homeError := os.UserHomeDir()
	if homeError != nil {
		return ".tael.yaml"
	}
	return filepath.Join(home, ".tael.yaml")
}

// readConfigFile loads ~/.tael.yaml. A missing or unreadable file is not an
// error — the config file is optional.
func readConfigFile() savedConfig {
	fileConfig := viper.New()
	fileConfig.SetConfigFile(configFilePath())
	fileConfig.SetConfigType("yaml")
	if readError := fileConfig.ReadInConfig(); readError != nil {
		return savedConfig{}
	}
	return savedConfig{
		Token:     fileConfig.GetString("token"),
		BaseURL:   fileConfig.GetString("base_url"),
		Workspace: fileConfig.GetString("workspace"),
	}
}

// writeConfigFile persists the config with 0600 permissions. The file is
// created (or tightened) to 0600 before the token lands in it, so the
// credential never briefly lives in a group- or world-readable file.
func writeConfigFile(config savedConfig) error {
	configPath := configFilePath()
	if directoryError := os.MkdirAll(filepath.Dir(configPath), 0o700); directoryError != nil {
		return fmt.Errorf("create config directory: %w", directoryError)
	}
	if secureError := ensureSecureConfigFile(configPath); secureError != nil {
		return secureError
	}

	fileConfig := viper.New()
	fileConfig.SetConfigFile(configPath)
	fileConfig.SetConfigType("yaml")
	fileConfig.SetConfigPermissions(0o600)
	fileConfig.Set("token", config.Token)
	fileConfig.Set("base_url", config.BaseURL)
	fileConfig.Set("workspace", config.Workspace)
	if writeError := fileConfig.WriteConfig(); writeError != nil {
		return fmt.Errorf("save config: %w", writeError)
	}
	if chmodError := os.Chmod(configPath, 0o600); chmodError != nil {
		return fmt.Errorf("secure config file: %w", chmodError)
	}
	return nil
}

// ensureSecureConfigFile creates path with 0600 permissions if it does not
// exist, or repairs the permissions of an existing file.
func ensureSecureConfigFile(path string) error {
	info, statError := os.Stat(path)
	switch {
	case statError == nil:
		if info.Mode().Perm()&0o077 != 0 {
			if chmodError := os.Chmod(path, 0o600); chmodError != nil {
				return fmt.Errorf("tighten config permissions: %w", chmodError)
			}
		}
		return nil
	case errors.Is(statError, os.ErrNotExist):
		file, createError := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0o600)
		if createError != nil {
			return fmt.Errorf("create config file: %w", createError)
		}
		return file.Close()
	default:
		return fmt.Errorf("stat config file: %w", statError)
	}
}
