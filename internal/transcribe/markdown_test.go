package transcribe

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"anoted/internal/config"
	"anoted/internal/session"
)

func TestWriteMeetingMarkdown(t *testing.T) {
	dir := t.TempDir()
	started := time.Date(2026, 6, 23, 14, 30, 0, 0, time.FixedZone("EDT", -4*3600))
	meta := session.Metadata{
		StartedAt:    started,
		EndedAt:      started.Add(42 * time.Minute),
		Duration:     "42m0s",
		Provider:     session.ProviderGoogleMeet,
		Platform:     "windows",
		Backend:      "wasapi",
		AutoRecord:   false,
		Manual:       true,
		SystemDevice: "Speakers (Realtek)",
	}
	if err := session.WriteMetadataFile(dir, meta); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, TranscriptBaseName+".txt"), []byte("Hello meeting transcript."), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := config.Default().Transcription
	cfg.OutputFormats = []string{config.OutputFormatMD}
	if err := WriteMeetingMarkdown(dir, dir, cfg); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "transcript.md"))
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	if !strings.HasPrefix(content, "---\n") {
		t.Fatal("expected yaml frontmatter")
	}
	for _, want := range []string{
		"provider: google_meet",
		"platform: windows",
		"2026-06-23",
		"- meeting",
		"- google-meet",
		"- tuesday",
		"Hello meeting transcript.",
	} {
		if !strings.Contains(content, want) {
			t.Fatalf("missing %q in:\n%s", want, content)
		}
	}
}

func TestExtractMarkdownBody(t *testing.T) {
	raw := "---\ntags:\n  - meeting\n---\n\nLine one\nLine two\n"
	path := filepath.Join(t.TempDir(), "note.md")
	if err := os.WriteFile(path, []byte(raw), 0o644); err != nil {
		t.Fatal(err)
	}
	body, err := ExtractMarkdownBody(path)
	if err != nil {
		t.Fatal(err)
	}
	if body != "Line one\nLine two" {
		t.Fatalf("got %q", body)
	}
}

func TestPruneTranscriptFiles(t *testing.T) {
	dir := t.TempDir()
	for _, ext := range []string{".txt", ".srt", ".vtt", ".json"} {
		if err := os.WriteFile(filepath.Join(dir, TranscriptBaseName+ext), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	keep := map[string]bool{config.OutputFormatTXT: true}
	pruneTranscriptFiles(dir, keep, TranscriptBaseName)

	if _, err := os.Stat(filepath.Join(dir, TranscriptBaseName+".txt")); err != nil {
		t.Fatal("txt should remain")
	}
	for _, ext := range []string{".srt", ".vtt", ".json"} {
		if _, err := os.Stat(filepath.Join(dir, TranscriptBaseName+ext)); !os.IsNotExist(err) {
			t.Fatalf("expected %s removed", ext)
		}
	}
}

func TestEffectiveWhisperFormatsForMarkdownOnly(t *testing.T) {
	got := effectiveWhisperFormats([]string{config.OutputFormatMD})
	if len(got) != 1 || got[0] != config.OutputFormatTXT {
		t.Fatalf("got %v", got)
	}
}

func TestRemoveTemporaryTxt(t *testing.T) {
	dir := t.TempDir()
	txt := filepath.Join(dir, TranscriptBaseName+".txt")
	if err := os.WriteFile(txt, []byte("body"), 0o644); err != nil {
		t.Fatal(err)
	}
	removeTemporaryTxt(dir, []string{config.OutputFormatMD}, TranscriptBaseName)
	if _, err := os.Stat(txt); !os.IsNotExist(err) {
		t.Fatal("expected temporary txt removed")
	}
}

func TestWhisperOutputFormatArg(t *testing.T) {
	if got := whisperOutputFormatArg([]string{config.OutputFormatSRT}); got != "srt" {
		t.Fatalf("got %q", got)
	}
	if got := whisperOutputFormatArg([]string{config.OutputFormatTXT, config.OutputFormatSRT}); got != "all" {
		t.Fatalf("got %q", got)
	}
}
