package transcribe

import (
	"encoding/json"
	"strings"
	"testing"
)

var sampleSegments = []Segment{
	{Start: 0, End: 2.5, Text: " Hola, buenos días."},
	{Start: 2.5, End: 5, Text: " ¿Qué tal?"},
	{Start: 5, End: 7, Text: "   "}, // blank: must be skipped, not emitted as an empty cue
	{Start: 3661.5, End: 3665.25, Text: " Pasada la hora."},
}

func TestFormatTimestampSeparators(t *testing.T) {
	// SRT uses a comma before milliseconds and VTT a period; getting this wrong
	// produces a file players silently refuse to load.
	if got := formatTimestamp(3661.5, ','); got != "01:01:01,500" {
		t.Fatalf("SRT timestamp = %q, want 01:01:01,500", got)
	}
	if got := formatTimestamp(3661.5, '.'); got != "01:01:01.500" {
		t.Fatalf("VTT timestamp = %q, want 01:01:01.500", got)
	}
	if got := formatTimestamp(0, ','); got != "00:00:00,000" {
		t.Fatalf("zero timestamp = %q", got)
	}
	// Rounding, not truncation, so a cue never starts before the previous ends.
	if got := formatTimestamp(1.9999, ','); got != "00:00:02,000" {
		t.Fatalf("rounding = %q, want 00:00:02,000", got)
	}
}

func TestRenderSRT(t *testing.T) {
	out := RenderSRT(sampleSegments)
	if !strings.HasPrefix(out, "1\n00:00:00,000 --> 00:00:02,500\nHola, buenos días.\n\n") {
		t.Fatalf("unexpected first cue:\n%s", out)
	}
	// Blank segments must not consume an index.
	if strings.Contains(out, "\n3\n00:00:05") {
		t.Fatalf("blank segment produced a cue:\n%s", out)
	}
	if !strings.Contains(out, "3\n01:01:01,500 --> 01:01:05,250\nPasada la hora.") {
		t.Fatalf("hour-plus cue missing or misnumbered:\n%s", out)
	}
}

func TestRenderVTTHasHeader(t *testing.T) {
	out := RenderVTT(sampleSegments)
	if !strings.HasPrefix(out, "WEBVTT\n\n") {
		t.Fatalf("missing WEBVTT header:\n%s", out)
	}
	if strings.Contains(out, ",") && strings.Contains(out, "-->") {
		// commas in the text are fine; commas in a timestamp are not
		for _, line := range strings.Split(out, "\n") {
			if strings.Contains(line, "-->") && strings.Contains(line, ",") {
				t.Fatalf("VTT cue uses a comma separator: %q", line)
			}
		}
	}
}

func TestRenderTXTSkipsBlanks(t *testing.T) {
	out := RenderTXT(sampleSegments)
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 3 {
		t.Fatalf("got %d lines, want 3 (blank segment dropped): %q", len(lines), out)
	}
	if lines[0] != "Hola, buenos días." {
		t.Fatalf("leading whitespace not trimmed: %q", lines[0])
	}
}

func TestRenderJSONShape(t *testing.T) {
	b, err := RenderJSON(sampleSegments, "es")
	if err != nil {
		t.Fatalf("RenderJSON: %v", err)
	}
	var got jsonTranscript
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	if got.Language != "es" {
		t.Fatalf("language = %q, want es", got.Language)
	}
	if len(got.Segments) != len(sampleSegments) {
		t.Fatalf("segments = %d, want %d", len(got.Segments), len(sampleSegments))
	}
	if got.Segments[0].ID != 0 || got.Segments[1].ID != 1 {
		t.Fatalf("segment ids not sequential: %+v", got.Segments[:2])
	}
	if !strings.Contains(got.Text, "buenos días") {
		t.Fatalf("full text missing content: %q", got.Text)
	}
}

func TestRenderEmptySegments(t *testing.T) {
	// A recording of pure silence must still produce well-formed files.
	if got := RenderTXT(nil); got != "" {
		t.Fatalf("RenderTXT(nil) = %q", got)
	}
	if got := RenderSRT(nil); got != "" {
		t.Fatalf("RenderSRT(nil) = %q", got)
	}
	if got := RenderVTT(nil); got != "WEBVTT\n\n" {
		t.Fatalf("RenderVTT(nil) = %q", got)
	}
	b, err := RenderJSON(nil, "")
	if err != nil {
		t.Fatalf("RenderJSON(nil): %v", err)
	}
	if !strings.Contains(string(b), `"segments": []`) {
		t.Fatalf("empty segments should marshal as [], got %s", b)
	}
}
