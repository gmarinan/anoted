package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefault(t *testing.T) {
	cfg := Default()
	if cfg.AutoRecord {
		t.Fatal("auto_record must be false by default")
	}
	if !cfg.AutoRecordRequiresConfirmation {
		t.Fatal("auto_record_requires_confirmation must be true by default")
	}
	if cfg.Audio.SampleRate != 48000 {
		t.Fatalf("expected sample rate 48000, got %d", cfg.Audio.SampleRate)
	}
}

func TestLoadMissingFile(t *testing.T) {
	cfg, err := Load(filepath.Join(t.TempDir(), "missing.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.OutputDir != Default().OutputDir {
		t.Fatalf("unexpected output dir: %s", cfg.OutputDir)
	}
}

func TestSaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	want := Default()
	want.AutoRecord = true

	if err := Save(path, want); err != nil {
		t.Fatal(err)
	}

	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if !got.AutoRecord {
		t.Fatal("expected auto_record true")
	}
}

func TestExpandPath(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	got, err := ExpandPath("~/Music/anoted")
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(home, "Music/anoted")
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}
