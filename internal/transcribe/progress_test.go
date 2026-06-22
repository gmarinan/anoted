package transcribe

import (
	"testing"
	"time"
)

func TestParseCppProgressLine(t *testing.T) {
	pct, ok := ParseCppProgressLine("whisper_print_progress_callback: progress =  65%")
	if !ok || pct != 65 {
		t.Fatalf("got %v %v, want 65 true", pct, ok)
	}
	_, ok = ParseCppProgressLine("noise")
	if ok {
		t.Fatal("expected false for unrelated line")
	}
}

func TestParseSegmentLine(t *testing.T) {
	dur := 60 * time.Second
	line := "[00:12.000 --> 00:18.000] hello world"
	pct, seg, ok := ParseSegmentLine(line, dur)
	if !ok || seg != line {
		t.Fatalf("unexpected parse: %v %q %v", pct, seg, ok)
	}
	if pct < 29 || pct > 31 {
		t.Fatalf("percent out of range: %v", pct)
	}
}

func TestParseTqdmProgressLine(t *testing.T) {
	line := " 65%|██████▌   | 12345/67890 [00:30<00:15, 412.00frames/s]"
	pct, ok := ParseTqdmProgressLine(line)
	if !ok || pct != 65 {
		t.Fatalf("got %v %v, want 65 true", pct, ok)
	}
}

func TestComputeETA(t *testing.T) {
	eta := ComputeETA(50, 30*time.Second)
	if eta < 25*time.Second || eta > 35*time.Second {
		t.Fatalf("eta out of range: %v", eta)
	}
	if ComputeETA(2, time.Second) != 0 {
		t.Fatal("expected zero eta for low percent")
	}
}

func TestFormatETA(t *testing.T) {
	if got := FormatETA(15 * time.Second); got != "0m 15s" {
		t.Fatalf("got %q", got)
	}
	if got := FormatETA(90 * time.Second); got != "1m 30s" {
		t.Fatalf("got %q", got)
	}
}
