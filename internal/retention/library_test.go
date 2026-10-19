package retention

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"anoted/internal/config"
	"anoted/internal/session"
)

func writeSession(t *testing.T, root, name string, files map[string]string) string {
	t.Helper()
	dir := filepath.Join(root, name)
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	for f, body := range files {
		if err := os.WriteFile(filepath.Join(dir, f), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestScanEmptyTranscriptKeepsAudio(t *testing.T) {
	root := t.TempDir()
	// What each backend leaves behind for a silent or unreadable wav.
	empty := writeSession(t, root, "empty", map[string]string{
		"recording.wav":   "wav",
		"transcript.txt":  "\n",
		"transcript.srt":  "",
		"transcript.vtt":  "WEBVTT\n\n",
		"transcript.json": `{"language":"en","text":"","segments":[]}`,
		"transcript.md":   "---\nprovider: teams\n---\n\n",
	})
	spoken := writeSession(t, root, "spoken", map[string]string{
		"recording.wav":  "wav",
		"transcript.txt": "",
		"transcript.vtt": "WEBVTT\n\n00:00:00.000 --> 00:00:01.000\nhello\n\n",
	})

	ended := time.Now().Add(-10 * 24 * time.Hour)
	recs := []session.Record{
		{Dir: empty, EndedAt: ended, Status: session.StatusStopped},
		{Dir: spoken, EndedAt: ended, Status: session.StatusStopped},
	}
	items := Scan(root, recs, config.TranscriptionConfig{}, nil)
	byDir := map[string]Item{}
	for _, it := range items {
		byDir[it.Dir] = it
	}
	if it := byDir[empty]; !it.HasTranscript || !it.EmptyTranscript {
		t.Fatalf("empty %+v", it)
	}
	if it := byDir[spoken]; !it.HasTranscript || it.EmptyTranscript {
		t.Fatalf("spoken %+v", it)
	}

	plan := BuildPlan(time.Now(), Policy{Enabled: true, KeepDays: 1, ProtectUntranscribed: true}, items)
	if got := paths(plan); len(got) != 1 || got[0] != AudioPath(spoken) {
		t.Fatalf("removed %v, want only the spoken session", got)
	}
	if plan.Protected != 1 {
		t.Fatalf("protected = %d, want the empty transcript", plan.Protected)
	}
	if u := Summarize(items); u.UntranscribedFiles != 1 {
		t.Fatalf("usage %+v", u)
	}
}

func TestScanCountsTranscriptInAnyFormat(t *testing.T) {
	root := t.TempDir()
	// Transcribed as srt, then the user switched output_formats to markdown.
	dir := writeSession(t, root, "old", map[string]string{
		"recording.wav":  "wav",
		"transcript.srt": "1\n00:00:00,000 --> 00:00:01,000\nhello\n\n",
	})
	tcfg := config.TranscriptionConfig{OutputFormats: []string{config.OutputFormatMD}}
	items := Scan(root, []session.Record{{Dir: dir}}, tcfg, nil)
	if len(items) != 1 || !items[0].transcribed() {
		t.Fatalf("items %+v", items)
	}
}

func TestScanActiveRowIsInUse(t *testing.T) {
	root := t.TempDir()
	dir := writeSession(t, root, "live", map[string]string{
		"recording.wav":  "wav",
		"transcript.txt": "old take",
	})
	recs := []session.Record{{Dir: dir, EndedAt: time.Now().Add(-48 * time.Hour), Status: session.StatusActive}}
	items := Scan(root, recs, config.TranscriptionConfig{}, nil)
	if len(items) != 1 || !items[0].InUse {
		t.Fatalf("items %+v", items)
	}
	if plan := BuildPlan(time.Now(), Policy{Enabled: true, KeepDays: 1}, items); len(plan.Remove) != 0 {
		t.Fatalf("active session audio planned for removal: %+v", plan.Remove)
	}
}

func TestTranscriptHasText(t *testing.T) {
	dir := t.TempDir()
	cases := []struct {
		name, body string
		want       bool
	}{
		{"a.txt", "  \n", false},
		{"b.txt", "hi", true},
		{"c.vtt", "WEBVTT\n\n", false},
		{"d.json", `{"text":"  ","segments":[{"text":" "}]}`, false},
		{"e.json", `{"text":"","segments":[{"text":"hey"}]}`, true},
		{"f.json", `{"text":`, false},
		{"g.md", "---\n{}\n---\n\n", false},
		{"h.md", "---\ndate: x\n---\n\nnotes\n", true},
		{"i.md", "no frontmatter", true},
	}
	for _, c := range cases {
		p := filepath.Join(dir, c.name)
		if err := os.WriteFile(p, []byte(c.body), 0o600); err != nil {
			t.Fatal(err)
		}
		if got := transcriptHasText(p); got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
	if transcriptHasText(filepath.Join(dir, "missing.txt")) {
		t.Error("missing file has no text")
	}
}

func TestPlanWithoutDropsBusyDirs(t *testing.T) {
	plan := Plan{Remove: []Removal{{Dir: "a"}, {Dir: "b"}}, Protected: 2}
	got := plan.Without(map[string]struct{}{"a": {}})
	if len(got.Remove) != 1 || got.Remove[0].Dir != "b" || got.Protected != 2 {
		t.Fatalf("%+v", got)
	}
	if len(plan.Remove) != 2 {
		t.Fatal("Without must not modify the original plan")
	}
}

func TestPolicyFromConfig(t *testing.T) {
	off := false
	got := PolicyFrom(config.RetentionConfig{AutoDelete: true, KeepDays: 7, KeepRecordings: 3, ProtectUntranscribed: &off})
	want := Policy{Enabled: true, KeepDays: 7, KeepRecordings: 3}
	if got != want {
		t.Fatalf("got %+v, want %+v", got, want)
	}
	if !PolicyFrom(config.RetentionConfig{}).ProtectUntranscribed {
		t.Fatal("a missing protect_untranscribed must protect")
	}
}
