package transcribe

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"anoted/internal/config"
	"anoted/internal/session"
)

func TestPostProcessMarkdownOnly(t *testing.T) {
	dir := t.TempDir()
	meta := session.Metadata{
		StartedAt: time.Now(),
		Provider:  session.ProviderGoogleMeet,
		Platform:  "linux",
	}
	if err := session.WriteMetadataFile(dir, meta); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, TranscriptBaseName+".txt"), []byte("meeting notes"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := config.Default().Transcription
	cfg.OutputFormats = []string{config.OutputFormatMD}

	res, err := postProcessTranscription(cfg, dir, dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Files) != 1 {
		t.Fatalf("expected 1 file, got %v", res.Files)
	}
	if _, err := os.Stat(filepath.Join(dir, "transcript.md")); err != nil {
		t.Fatal("expected transcript.md")
	}
	if _, err := os.Stat(filepath.Join(dir, TranscriptBaseName+".txt")); !os.IsNotExist(err) {
		t.Fatal("expected temporary transcript.txt removed")
	}
}
