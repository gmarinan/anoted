package tui

import (
	"os"
	"path/filepath"
	"testing"

	"anoted/internal/config"
)

func TestConfigFolderStartDirFromField(t *testing.T) {
	dir := t.TempDir()
	m := Model{
		configSection: 3,
		configCursor:  7, // output_dir in transcription section — verify index
		deps: Deps{
			Config: config.Config{
				Transcription: config.TranscriptionConfig{
					OutputDir: dir,
				},
			},
		},
	}
	// Find output_dir cursor dynamically
	fields := cfgFields(3)
	cursor := -1
	for i, f := range fields {
		if f.label == "output_dir" {
			cursor = i
			break
		}
	}
	if cursor < 0 {
		t.Fatal("output_dir field not found")
	}
	m.configCursor = cursor

	got := m.configFolderStartDir()
	want, _ := filepath.Abs(dir)
	absGot, _ := filepath.Abs(got)
	if absGot != want {
		t.Fatalf("got %q want %q", absGot, want)
	}
}

func TestConfigFolderStartDirFallbackHome(t *testing.T) {
	fields := cfgFields(0)
	cursor := -1
	for i, f := range fields {
		if f.label == "output_dir" {
			cursor = i
			break
		}
	}
	m := Model{
		configSection: 0,
		configCursor:  cursor,
		deps: Deps{Config: config.Config{
			OutputDir: "",
		}},
	}
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	if m.configFolderStartDir() != home {
		t.Fatalf("expected home %q", home)
	}
}
