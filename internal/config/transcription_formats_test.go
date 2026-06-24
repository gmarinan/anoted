package config

import "testing"

func TestNormalizeOutputFormats(t *testing.T) {
	got := NormalizeOutputFormats([]string{"TXT", "srt", "bogus", "srt"})
	want := []string{OutputFormatTXT, OutputFormatSRT}
	if len(got) != len(want) {
		t.Fatalf("got %v want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v want %v", got, want)
		}
	}
}

func TestNormalizeOutputFormatsEmpty(t *testing.T) {
	got := NormalizeOutputFormats(nil)
	if len(got) != 1 || got[0] != OutputFormatTXT {
		t.Fatalf("got %v", got)
	}
}

func TestDefaultOutputFormats(t *testing.T) {
	cfg := Default()
	got := NormalizeOutputFormats(cfg.Transcription.OutputFormats)
	if len(got) != 2 || got[0] != OutputFormatTXT || got[1] != OutputFormatSRT {
		t.Fatalf("got %v", got)
	}
	if cfg.Transcription.Markdown.Filename != "transcript.md" {
		t.Fatalf("filename %q", cfg.Transcription.Markdown.Filename)
	}
	if !cfg.Transcription.Markdown.MarkdownWeekdayClassEnabled() {
		t.Fatal("weekday class should default true")
	}
}

func TestWantsMarkdown(t *testing.T) {
	if !WantsMarkdown([]string{OutputFormatTXT, OutputFormatMD}) {
		t.Fatal("expected md wanted")
	}
	if WantsMarkdown([]string{OutputFormatTXT}) {
		t.Fatal("expected md not wanted")
	}
}
