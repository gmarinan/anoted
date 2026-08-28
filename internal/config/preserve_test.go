package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Every write from the Config tab re-marshalled the struct, which silently
// deleted every comment in the user's file — including incidental writes like
// choosing a file manager.
func TestSaveKeepsUserComments(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	original := `# My anoted setup — do not let the tool eat this.
auto_record: false

# Recordings live on the big disk.
output_dir: ~/Music/anoted
audio:
  sample_rate: 48000 # matches my interface
`
	if err := os.WriteFile(path, []byte(original), 0o600); err != nil {
		t.Fatalf("seed config: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	cfg.AutoRecord = true
	if err := Save(path, cfg); err != nil {
		t.Fatalf("Save: %v", err)
	}

	saved, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	body := string(saved)
	for _, comment := range []string{
		"do not let the tool eat this",
		"Recordings live on the big disk",
		"matches my interface",
	} {
		if !strings.Contains(body, comment) {
			t.Fatalf("comment %q was lost:\n%s", comment, body)
		}
	}

	// The value change must actually have been written.
	reloaded, err := Load(path)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if !reloaded.AutoRecord {
		t.Fatal("auto_record change was not saved")
	}
	if reloaded.Audio.SampleRate != 48000 {
		t.Fatalf("sample_rate = %d, want 48000", reloaded.Audio.SampleRate)
	}
}

func TestSaveWorksWithNoExistingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := Save(path, Default()); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if _, err := Load(path); err != nil {
		t.Fatalf("Load: %v", err)
	}
}

func TestSaveRejectsOutOfRangeValues(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")

	cfg := Default()
	cfg.Detection.PollIntervalMS = 1 // a pactl fork every millisecond
	if err := Save(path, cfg); err == nil {
		t.Fatal("a 1ms poll interval must be rejected")
	}

	cfg = Default()
	cfg.Audio.Channels = 7
	if err := Save(path, cfg); err == nil {
		t.Fatal("7 audio channels must be rejected")
	}

	cfg = Default()
	cfg.Detection.Mode = "telepathy"
	if err := Save(path, cfg); err == nil {
		t.Fatal("an unknown detection mode must be rejected")
	}

	// Nothing should have been written by the rejected saves.
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("an invalid config must not reach disk")
	}
}

func TestValidateRejectsBlankProviderPattern(t *testing.T) {
	cfg := Default()
	cfg.Detection.Providers["teams"] = ProviderConfig{Patterns: []string{"teams.microsoft.com", "  "}}
	err := cfg.Validate()
	if err == nil {
		t.Fatal("a blank pattern matches every window title and must be rejected")
	}
	if !strings.Contains(err.Error(), "teams") {
		t.Fatalf("error should name the provider: %v", err)
	}
}
