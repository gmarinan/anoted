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

func TestParseSegmentLineHours(t *testing.T) {
	// whisper switches to HH:MM:SS.mmm past the hour; without the hour field
	// progress froze for the tail of any meeting longer than 60 minutes.
	dur := 2 * time.Hour
	line := "[01:00:00.000 --> 01:30:00.000] second hour"
	pct, seg, ok := ParseSegmentLine(line, dur)
	if !ok || seg != line {
		t.Fatalf("unexpected parse: %v %q %v", pct, seg, ok)
	}
	if pct < 74 || pct > 76 {
		t.Fatalf("percent = %v, want ~75", pct)
	}
	// sub-hour form must still work
	if pct, _, ok := ParseSegmentLine("[00:30.000 --> 01:00.000] first", 2*time.Minute); !ok || pct < 49 || pct > 51 {
		t.Fatalf("sub-hour form broke: %v %v", pct, ok)
	}
}
