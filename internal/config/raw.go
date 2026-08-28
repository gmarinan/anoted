package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// ReadRaw returns the on-disk config file contents. A missing file yields
// YAML marshaled from defaults.
func ReadRaw(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			def := Default()
			def.applyDefaults()
			out, mErr := yaml.Marshal(&def)
			if mErr != nil {
				return "", fmt.Errorf("marshal default config: %w", mErr)
			}
			return string(out), nil
		}
		return "", fmt.Errorf("read config %s: %w", path, err)
	}
	return string(data), nil
}

// SaveRaw validates YAML, writes it to path, and returns the parsed config.
func SaveRaw(path string, content string) (Config, error) {
	var cfg Config
	if err := yaml.Unmarshal([]byte(content), &cfg); err != nil {
		return Config{}, fmt.Errorf("invalid yaml: %w", err)
	}
	cfg.applyDefaults()

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return Config{}, fmt.Errorf("create config dir: %w", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		return Config{}, fmt.Errorf("write config %s: %w", path, err)
	}
	return cfg, nil
}
