package config

import (
	"os"
	"path/filepath"
)

const LegacyAppName = "meetctl"

// LegacyConfigDir returns the pre-rename user config directory, if present.
func LegacyConfigDir() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, LegacyAppName), nil
}

// LegacyConfigPath returns the pre-rename config file path.
func LegacyConfigPath() (string, error) {
	dir, err := LegacyConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, DefaultFilename), nil
}

// LegacyDataDir returns the pre-rename XDG data directory for managed assets.
func LegacyDataDir() string {
	if xdg := os.Getenv("XDG_DATA_HOME"); xdg != "" {
		return filepath.Join(xdg, LegacyAppName)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".local", "share", LegacyAppName)
	}
	return filepath.Join(home, ".local", "share", LegacyAppName)
}
