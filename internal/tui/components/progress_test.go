package components

import (
	"strings"
	"testing"
	"time"
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

func TestTXStatusLabel(t *testing.T) {
	yes := TXStatusLabel("yes")
	if !strings.Contains(yes, "yes") {
		t.Fatalf("yes label: %q", yes)
	}
	no := TXStatusLabel("no")
	if !strings.Contains(no, "no") {
		t.Fatalf("no label: %q", no)
	}
}
