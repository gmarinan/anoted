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
	AppName         = "anoted"
)

// Config holds application settings loaded from YAML.
type Config struct {
	SetupCompleted                 bool                `yaml:"setup_completed"`
	AutoRecord                     bool                `yaml:"auto_record"`
	AutoRecordRequiresConfirmation bool                `yaml:"auto_record_requires_confirmation"`
	OutputDir                      string              `yaml:"output_dir"`
	Audio                          AudioConfig         `yaml:"audio"`
	Detection                      DetectConfig        `yaml:"detection"`
	Transcription                  TranscriptionConfig `yaml:"transcription"`
	Desktop                        DesktopConfig       `yaml:"desktop"`
	Privacy                        PrivacyConfig       `yaml:"privacy"`
}

type DesktopConfig struct {
	// Opener: auto, xdg-open, dolphin, nautilus, thunar, pcmanfm, nemo, caja, custom
	Opener string `yaml:"opener"`
	// OpenCommand is used when opener is custom, e.g. ["dolphin", "{path}"]
	OpenCommand []string `yaml:"open_command"`
	// FileOpener overrides how recording.wav is opened (default: xdg-open).
	FileOpener string `yaml:"file_opener"`
	// WMClass is the window class for autostart terminal (Hyprland/Sway rules).
	WMClass string `yaml:"wm_class"`
	// AutostartTerminal launches anoted inside this terminal, e.g. ["alacritty", "--class", "anoted", "-e"].
	AutostartTerminal []string `yaml:"autostart_terminal"`
}

type TranscriptionConfig struct {
	AutoAfterRecording bool                        `yaml:"auto_after_recording"`
	Binary             string                      `yaml:"binary"` // empty = auto-detect in PATH
	Backend            string                      `yaml:"backend"`
	Model              string                      `yaml:"model"`
	Language           string                      `yaml:"language"`
	Device             string                      `yaml:"device"`
	GPULayers          int                         `yaml:"gpu_layers"`
	ModelPath          string                      `yaml:"model_path"`
	OutputFormats      []string                    `yaml:"output_formats"`
	OutputDir          string                      `yaml:"output_dir"` // empty = same folder as recording
	Markdown           TranscriptionMarkdownConfig `yaml:"markdown"`
}

// TranscriptionMarkdownConfig controls Obsidian-style meeting notes.
type TranscriptionMarkdownConfig struct {
	Filename     string   `yaml:"filename"`
	Tags         []string `yaml:"tags"`
	CSSClasses   []string `yaml:"cssclasses"`
	WeekdayClass *bool    `yaml:"weekday_class"`
}

// MarkdownWeekdayClassEnabled reports whether to add weekday cssclass (default true).
func (m TranscriptionMarkdownConfig) MarkdownWeekdayClassEnabled() bool {
	if m.WeekdayClass == nil {
		return true
	}
	return *m.WeekdayClass
}

type AudioConfig struct {
	LinuxBackendPriority   []string `yaml:"linux_backend_priority"`
	WindowsBackendPriority []string `yaml:"windows_backend_priority"`
	SampleRate             int      `yaml:"sample_rate"`
	Channels               int      `yaml:"channels"`
	SystemMonitor          string   `yaml:"system_monitor"` // empty = auto (default sink monitor)
	Microphone             string   `yaml:"microphone"`     // empty = auto (default source)
	// LevelLatencyMsec and LevelProcessTimeMsec tune parec on Linux (lower = smoother bars, more CPU).
	LevelLatencyMsec     int    `yaml:"level_latency_msec"`
	LevelProcessTimeMsec int    `yaml:"level_process_time_msec"`
	LevelUITickMS        int    `yaml:"level_ui_tick_ms"`
	LevelPreset          string `yaml:"level_preset"` // responsive, balanced, economy, custom
}

type DetectConfig struct {
	PollIntervalMS    int                       `yaml:"poll_interval_ms"`
	MeetingEndGraceMS int                       `yaml:"meeting_end_grace_ms"`
	Mode              string                    `yaml:"mode"`        // mic, window, both, none
	WindowTool        string                    `yaml:"window_tool"` // auto, xdotool, wmctrl, none (for window/both modes)
	Providers         map[string]ProviderConfig `yaml:"providers"`
}

type ProviderConfig struct {
	Patterns []string `yaml:"patterns"`
}

type PrivacyConfig struct {
	ShowRecordingIndicator       bool `yaml:"show_recording_indicator"`
	TrayIndicator                bool `yaml:"tray_indicator"`
	RequireManualConsentFirstRun bool `yaml:"require_manual_consent_first_run"`
}

// Default returns the built-in default configuration.
func Default() Config {
	return Config{
		AutoRecord:                     false,
		AutoRecordRequiresConfirmation: true,
		OutputDir:                      "~/Music/anoted",
		Audio: AudioConfig{
			LinuxBackendPriority:   []string{"pipewire", "pulseaudio", "ffmpeg"},
			WindowsBackendPriority: []string{"wasapi"},
			SampleRate:             48000,
			Channels:               2,
			LevelLatencyMsec:       50,
			LevelProcessTimeMsec:   20,
			LevelUITickMS:          33,
			LevelPreset:            LevelPresetResponsive,
		},
		Detection: DetectConfig{
			PollIntervalMS:    2000,
			MeetingEndGraceMS: 6000,
			Mode:              "mic",
			WindowTool:        "auto",
			Providers: map[string]ProviderConfig{
				"google_meet": {
					Patterns: []string{"meet.google.com", "Google Meet", "Meet -", "Meet |"},
				},
				"teams": {
					// "| Meet" is a Google Meet tab title, not a Teams one; it
					// used to sit here and made every Meet call match both
					// providers at once.
					Patterns: []string{"teams.microsoft.com", "Meeting with", "In a call"},
				},
			},
		},
		Transcription: TranscriptionConfig{
			AutoAfterRecording: false,
			Backend:            "auto",
			Model:              "turbo",
			Device:             "auto",
			GPULayers:          0,
			OutputFormats:      []string{OutputFormatTXT, OutputFormatSRT},
			Markdown: TranscriptionMarkdownConfig{
				Filename:   "transcript.md",
				Tags:       []string{"meeting"},
				CSSClasses: []string{"meeting"},
			},
		},
		Desktop: DesktopConfig{
			Opener:  "auto",
			WMClass: "anoted",
		},
		Privacy: PrivacyConfig{
			ShowRecordingIndicator:       true,
			TrayIndicator:                true,
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
//
// Values that are out of range are rejected here rather than at the point of
// use: a bad poll interval or sample rate otherwise surfaced as an ffmpeg
// failure in the middle of a meeting.
func Save(path string, cfg Config) error {
	cfg.applyDefaults()
	if err := cfg.Validate(); err != nil {
		return err
	}
	data := marshalPreservingComments(path, cfg)
	if data == nil {
		return fmt.Errorf("marshal config")
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("write config %s: %w", tmp, err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
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
		if err := MigrateLegacyIfNeeded(path); err != nil {
			return "", err
		}
		return path, nil
	} else if !errors.Is(statErr, os.ErrNotExist) {
		return "", fmt.Errorf("stat config %s: %w", path, statErr)
	}

	legacyPath, err := LegacyConfigPath()
	if err == nil {
		if _, err := os.Stat(legacyPath); err == nil {
			cfg, loadErr := Load(legacyPath)
			if loadErr == nil {
				if saveErr := Save(path, cfg); saveErr == nil {
					return path, nil
				}
			}
		}
	}

	if err := Save(path, platformDefault()); err != nil {
		return "", err
	}
	return path, nil
}

func platformDefault() Config {
	return Default()
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
	if c.Audio.LevelLatencyMsec == 0 {
		c.Audio.LevelLatencyMsec = def.Audio.LevelLatencyMsec
	}
	if c.Audio.LevelProcessTimeMsec == 0 {
		c.Audio.LevelProcessTimeMsec = def.Audio.LevelProcessTimeMsec
	}
	if c.Audio.LevelUITickMS == 0 {
		c.Audio.LevelUITickMS = def.Audio.LevelUITickMS
	}
	if c.Audio.LevelPreset == "" {
		c.Audio.LevelPreset = InferLevelPreset(c.Audio.LevelLatencyMsec, c.Audio.LevelProcessTimeMsec, c.Audio.LevelUITickMS)
	} else if c.Audio.LevelPreset != LevelPresetCustom && c.Audio.LevelPreset != LevelPresetOff {
		_ = ApplyLevelPreset(c, c.Audio.LevelPreset)
	}
	if c.Detection.PollIntervalMS == 0 {
		c.Detection.PollIntervalMS = def.Detection.PollIntervalMS
	}
	if c.Detection.MeetingEndGraceMS == 0 {
		c.Detection.MeetingEndGraceMS = def.Detection.MeetingEndGraceMS
	}
	if c.Detection.Mode == "" {
		c.Detection.Mode = def.Detection.Mode
	}
	if c.Detection.WindowTool == "" {
		c.Detection.WindowTool = def.Detection.WindowTool
	}
	// Config is passed by value but a map header is shared, so mutating the
	// caller's map here was an unsynchronized write racing View's reads on the
	// Update loop. Work on a copy instead — Save applies defaults on every save,
	// so this ran constantly, not only when providers changed.
	if c.Detection.Providers == nil {
		c.Detection.Providers = cloneProviders(def.Detection.Providers)
	} else {
		providers := cloneProviders(c.Detection.Providers)
		mergeProviderPatterns(providers, def.Detection.Providers)
		c.Detection.Providers = providers
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
	if len(c.Transcription.OutputFormats) == 0 {
		c.Transcription.OutputFormats = append([]string(nil), def.Transcription.OutputFormats...)
	}
	c.Transcription.OutputFormats = NormalizeOutputFormats(c.Transcription.OutputFormats)
	if c.Transcription.Markdown.Filename == "" {
		c.Transcription.Markdown.Filename = def.Transcription.Markdown.Filename
	}
	if len(c.Transcription.Markdown.Tags) == 0 {
		c.Transcription.Markdown.Tags = append([]string(nil), def.Transcription.Markdown.Tags...)
	}
	if len(c.Transcription.Markdown.CSSClasses) == 0 {
		c.Transcription.Markdown.CSSClasses = append([]string(nil), def.Transcription.Markdown.CSSClasses...)
	}
	if c.Desktop.Opener == "" {
		c.Desktop.Opener = def.Desktop.Opener
	}
	if c.Desktop.WMClass == "" {
		c.Desktop.WMClass = def.Desktop.WMClass
	}
}

// mergeProviderPatterns seeds defaults for providers the user has never
// configured. It deliberately does not re-add individual patterns to a provider
// the user already has: doing so ran on both Load and Save, which made deleting
// a pattern in the Config tab a permanent no-op — and with auto-record on, a
// false-positive pattern was impossible to remove.
//
// An explicitly empty pattern list is a real choice (match nothing), so it is
// left alone; only a missing provider key is seeded.
func cloneProviders(src map[string]ProviderConfig) map[string]ProviderConfig {
	out := make(map[string]ProviderConfig, len(src))
	for name, p := range src {
		out[name] = ProviderConfig{Patterns: append([]string(nil), p.Patterns...)}
	}
	return out
}

func mergeProviderPatterns(dst map[string]ProviderConfig, defaults map[string]ProviderConfig) {
	for name, defP := range defaults {
		if _, ok := dst[name]; ok {
			continue
		}
		dst[name] = ProviderConfig{Patterns: append([]string(nil), defP.Patterns...)}
	}
}
