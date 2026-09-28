package components

import (
	"strings"
	"testing"
	"time"

	"anoted/internal/session"
)

func TestTranscribeProgressBar(t *testing.T) {
	out := TranscribeProgressBar(65, 10, 15*time.Second, false)
	if !strings.Contains(out, "65%") {
		t.Fatalf("missing percent: %q", out)
	}
	if !strings.Contains(out, "ETA") {
		t.Fatalf("missing ETA: %q", out)
	}
	if !strings.Contains(out, "█") || !strings.Contains(out, "░") {
		t.Fatalf("missing bar chars: %q", out)
	}
}

// The boundary cell renders an eighth-block partial so the bar moves smoothly
// between whole cells.
func TestTranscribeProgressBarPartialCell(t *testing.T) {
	out := TranscribeProgressBar(65, 10, 0, false)
	// 65% of 10 cells = 6.5 cells: 6 full blocks then a half block.
	if !strings.Contains(out, "██████▌") {
		t.Fatalf("expected partial boundary cell: %q", out)
	}
}

func TestTXStatusLabel(t *testing.T) {
	yes := TXStatusLabel("yes")
	if !strings.Contains(yes, "done") {
		t.Fatalf("yes label: %q", yes)
	}
	no := TXStatusLabel("no")
	if !strings.Contains(no, "none") {
		t.Fatalf("no label: %q", no)
	}
}

func TestFormatTXColumnShowsQueuePosition(t *testing.T) {
	v := SessionsView{
		Width:           160,
		TranscribeQueue: []string{"/s/a", "/s/b"},
		Artifacts: map[string]SessionArtifacts{
			"/s/b": {HasAudio: true, HasTranscript: true},
		},
	}
	got := v.formatTXColumn(session.Record{Dir: "/s/b"}, 160)
	if !strings.Contains(got, "queue 2") {
		t.Fatalf("queued session with a transcript rendered %q", got)
	}
}
