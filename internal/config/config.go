package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	DefaultFilename = "config.yaml"
	AppName         = "meetctl"
)

// Config holds application settings loaded from YAML.
type Config struct {
	SetupCompleted                 bool          `yaml:"setup_completed"`
	AutoRecord                     bool          `yaml:"auto_record"`
	AutoRecordRequiresConfirmation bool          `yaml:"auto_record_requires_confirmation"`
	OutputDir                      string                `yaml:"output_dir"`
	Audio                          AudioConfig           `yaml:"audio"`
	Detection                      DetectConfig          `yaml:"detection"`
	Transcription                  TranscriptionConfig   `yaml:"transcription"`
	Privacy                        PrivacyConfig         `yaml:"privacy"`
}

type TranscriptionConfig struct {
	AutoAfterRecording bool   `yaml:"auto_after_recording"`
	Binary             string `yaml:"binary"`      // empty = auto-detect in PATH
	Backend            string `yaml:"backend"`     // auto, openai-whisper, whisper-cpp
	Model              string `yaml:"model"`       // tiny, base, small, medium, large
	Language           string `yaml:"language"`    // empty = auto-detect
	Device             string `yaml:"device"`      // cpu, cuda, auto
	GPULayers          int    `yaml:"gpu_layers"`  // whisper.cpp -ngl (0 = CPU only)
	ModelPath          string `yaml:"model_path"`  // whisper.cpp ggml model file
}

type AudioConfig struct {
	LinuxBackendPriority   []string `yaml:"linux_backend_priority"`
	WindowsBackendPriority []string `yaml:"windows_backend_priority"`
	SampleRate             int      `yaml:"sample_rate"`
	Channels               int      `yaml:"channels"`
	SystemMonitor          string   `yaml:"system_monitor"` // empty = auto (default sink monitor)
	Microphone             string   `yaml:"microphone"`     // empty = auto (default source)
}

type DetectConfig struct {
	PollIntervalMS int                       `yaml:"poll_interval_ms"`
	Mode           string                    `yaml:"mode"`        // mic, window, both, none
	WindowTool     string                    `yaml:"window_tool"` // auto, xdotool, wmctrl, none (for window/both modes)
	Providers      map[string]ProviderConfig `yaml:"providers"`
}

type ProviderConfig struct {
	Patterns []string `yaml:"patterns"`
}

type PrivacyConfig struct {
	ShowRecordingIndicator       bool `yaml:"show_recording_indicator"`
	RequireManualConsentFirstRun bool `yaml:"require_manual_consent_first_run"`
}

// Default returns the built-in default configuration.
func Default() Config {
	return Config{
		AutoRecord:                     false,
		AutoRecordRequiresConfirmation: true,
		OutputDir:                      "~/Music/meetctl",
		Audio: AudioConfig{
			LinuxBackendPriority:   []string{"pipewire", "pulseaudio", "ffmpeg"},
			WindowsBackendPriority: []string{"wasapi"},
			SampleRate:             48000,
			Channels:               2,
		},
		Detection: DetectConfig{
			PollIntervalMS: 2000,
			Mode:           "mic",
			WindowTool:     "auto",
			Providers: map[string]ProviderConfig{
				"google_meet": {
					Patterns: []string{"meet.google.com", "Google Meet", "Meet -", "Meet |"},
				},
				"teams": {
					Patterns: []string{"teams.microsoft.com", "Microsoft Teams", "Teams"},
				},
			},
		},
		Transcription: TranscriptionConfig{
			AutoAfterRecording: false,
			Backend:            "auto",
			Model:              "base",
			Device:             "auto",
			GPULayers:          0,
		},
		Privacy: PrivacyConfig{
			ShowRecordingIndicator:       true,
			RequireManualConsentFirstRun: true,
		},
	}
}

// ConfigDir returns the per-user configuration directory.
func ConfigDir() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve user config dir: %w", err)
	}
	return filepath.Join(dir, AppName), nil
}

// ConfigPath returns the default config file path.
func ConfigPath() (string, error) {
	dir, err := ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, DefaultFilename), nil
}

// Load reads configuration from path. Missing file returns defaults.
func Load(path string) (Config, error) {
	cfg := Default()

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return cfg, nil
		}
		return cfg, fmt.Errorf("read config %s: %w", path, err)
	}

	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("parse config %s: %w", path, err)
	}

	cfg.applyDefaults()
	return cfg, nil
}

// LoadDefault loads from the standard user config path.
func LoadDefault() (Config, string, error) {
	path, err := ConfigPath()
	if err != nil {
		return Config{}, "", err
	}
	cfg, err := Load(path)
	return cfg, path, err
}

// Save writes configuration to path, creating parent directories as needed.
func Save(path string, cfg Config) error {
	cfg.applyDefaults()
	data, err := yaml.Marshal(&cfg)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write config %s: %w", path, err)
	}
	return nil
}

// EnsureDefault creates the default config file if it does not exist.
func EnsureDefault() (string, error) {
	path, err := ConfigPath()
	if err != nil {
		return "", err
	}
	if _, statErr := os.Stat(path); statErr == nil {
		return path, nil
	} else if !errors.Is(statErr, os.ErrNotExist) {
		return "", fmt.Errorf("stat config %s: %w", path, statErr)
	}
	if err := Save(path, Default()); err != nil {
		return "", err
	}
	return path, nil
}

// ExpandPath expands ~ in paths to the user home directory.
func ExpandPath(path string) (string, error) {
	if path == "" {
		return "", nil
	}
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve home dir: %w", err)
		}
		return filepath.Join(home, path[2:]), nil
	}
	return path, nil
}

// ResolvedOutputDir returns the expanded output directory.
func (c Config) ResolvedOutputDir() (string, error) {
	return ExpandPath(c.OutputDir)
}

func (c *Config) applyDefaults() {
	def := Default()
	if c.OutputDir == "" {
		c.OutputDir = def.OutputDir
	}
	if c.Audio.SampleRate == 0 {
		c.Audio.SampleRate = def.Audio.SampleRate
	}
	if c.Audio.Channels == 0 {
		c.Audio.Channels = def.Audio.Channels
	}
	if len(c.Audio.LinuxBackendPriority) == 0 {
		c.Audio.LinuxBackendPriority = def.Audio.LinuxBackendPriority
	}
	if len(c.Audio.WindowsBackendPriority) == 0 {
		c.Audio.WindowsBackendPriority = def.Audio.WindowsBackendPriority
	}
	if c.Detection.PollIntervalMS == 0 {
		c.Detection.PollIntervalMS = def.Detection.PollIntervalMS
	}
	if c.Detection.Mode == "" {
		c.Detection.Mode = def.Detection.Mode
	}
	if c.Detection.WindowTool == "" {
		c.Detection.WindowTool = def.Detection.WindowTool
	}
	if c.Detection.Providers == nil {
		c.Detection.Providers = def.Detection.Providers
	} else {
		mergeProviderPatterns(c.Detection.Providers, def.Detection.Providers)
	}
	if c.Transcription.Backend == "" {
		c.Transcription.Backend = def.Transcription.Backend
	}
	if c.Transcription.Model == "" {
		c.Transcription.Model = def.Transcription.Model
	}
	if c.Transcription.Device == "" {
		c.Transcription.Device = def.Transcription.Device
	}
}

func mergeProviderPatterns(dst map[string]ProviderConfig, defaults map[string]ProviderConfig) {
	for name, defP := range defaults {
		cur, ok := dst[name]
		if !ok || len(cur.Patterns) == 0 {
			dst[name] = ProviderConfig{Patterns: append([]string(nil), defP.Patterns...)}
			continue
		}
		seen := make(map[string]bool, len(cur.Patterns))
		for _, p := range cur.Patterns {
			seen[p] = true
		}
		for _, p := range defP.Patterns {
			if !seen[p] {
				cur.Patterns = append(cur.Patterns, p)
			}
		}
		dst[name] = cur
	}
}
