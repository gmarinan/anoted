package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLegacyConfigPath(t *testing.T) {
	got, err := LegacyConfigPath()
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(filepath.Dir(got)) != LegacyAppName {
		t.Fatalf("got %q", got)
	}
}

func TestMergeLegacyConfig(t *testing.T) {
	current := Default()
	legacy := Default()
	legacy.SetupCompleted = true
	legacy.Transcription.Model = "large"
	legacy.Transcription.Device = "cuda"
	legacy.Transcription.Binary = "/tmp/legacy-whisper"

	if !mergeLegacyConfig(&current, legacy) {
		t.Fatal("expected merge from completed legacy setup")
	}
	if !current.SetupCompleted {
		t.Fatal("setup not completed")
	}
	if current.Transcription.Model != "large" {
		t.Fatalf("model %q", current.Transcription.Model)
	}
	if current.Transcription.Binary != legacy.Transcription.Binary {
		t.Fatalf("binary %q", current.Transcription.Binary)
	}

	current = Default()
	current.SetupCompleted = true
	current.Transcription.Binary = ""
	bin := filepath.Join(t.TempDir(), "whisper")
	if err := os.WriteFile(bin, []byte("stub"), 0o755); err != nil {
		t.Fatal(err)
	}
	legacy.Transcription.Binary = bin

	if !mergeLegacyConfig(&current, legacy) {
		t.Fatal("expected binary merge")
	}
	if current.Transcription.Binary != bin {
		t.Fatalf("binary %q", current.Transcription.Binary)
	}
}
