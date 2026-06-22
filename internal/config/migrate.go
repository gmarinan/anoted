package config

import (
	"errors"
	"os"
)

// MigrateLegacyIfNeeded merges meetctl-era settings into the current config when
// anoted was started with a fresh default file after the rename.
func MigrateLegacyIfNeeded(currentPath string) error {
	legacyPath, err := LegacyConfigPath()
	if err != nil {
		return err
	}
	if _, err := os.Stat(legacyPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}

	current, err := Load(currentPath)
	if err != nil {
		return err
	}
	legacy, err := Load(legacyPath)
	if err != nil {
		return err
	}
	if !mergeLegacyConfig(&current, legacy) {
		return nil
	}
	return Save(currentPath, current)
}

func mergeLegacyConfig(current *Config, legacy Config) bool {
	if !legacy.SetupCompleted {
		return false
	}

	changed := false
	if !current.SetupCompleted {
		current.Transcription = legacy.Transcription
		current.SetupCompleted = true
		changed = true
	}
	if current.Transcription.Binary == "" && legacy.Transcription.Binary != "" {
		if _, err := os.Stat(legacy.Transcription.Binary); err == nil {
			current.Transcription.Binary = legacy.Transcription.Binary
			changed = true
		}
	}
	return changed
}
