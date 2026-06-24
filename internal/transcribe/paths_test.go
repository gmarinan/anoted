package transcribe

import (
	"os"
	"path/filepath"
	"testing"

	"anoted/internal/config"
)

func TestOutputDirDefaultSession(t *testing.T) {
	session := "/tmp/recordings/2026-06-23_google_meet"
	got, err := OutputDir(config.TranscriptionConfig{}, session)
	if err != nil {
		t.Fatal(err)
	}
	if got != session {
		t.Fatalf("got %q want %q", got, session)
	}
}

func TestOutputDirCustom(t *testing.T) {
	base := t.TempDir()
	cfg := config.TranscriptionConfig{OutputDir: base}
	session := filepath.Join(t.TempDir(), "2026-06-23_16-30-05_google_meet")
	got, err := OutputDir(cfg, session)
	if err != nil {
		t.Fatal(err)
	}
	if got != base {
		t.Fatalf("got %q want flat %q", got, base)
	}
	if _, err := os.Stat(got); err != nil {
		t.Fatal(err)
	}
}

func TestOutputDirExpandHome(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	cfg := config.TranscriptionConfig{OutputDir: "~/vault/transcripts"}
	got, err := OutputDir(cfg, "/music/anoted/session_a")
	if err != nil {
		t.Fatal(err)
	}
	home, _ := os.UserHomeDir()
	want := filepath.Join(home, "vault/transcripts")
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestMarkdownFilenameUniqueInSharedDir(t *testing.T) {
	cfg := config.TranscriptionConfig{OutputDir: "/vault/meetings"}
	session := "/recordings/2026-06-23_16-30-05_google_meet"
	got := markdownFilename(cfg, session)
	want := "2026-06-23_16-30-05_google_meet.md"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestMarkdownFilenameDefaultInSessionDir(t *testing.T) {
	cfg := config.Default().Transcription
	session := "/recordings/2026-06-23_16-30-05_google_meet"
	got := markdownFilename(cfg, session)
	if got != "transcript.md" {
		t.Fatalf("got %q want transcript.md", got)
	}
}
