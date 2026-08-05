package transcribe

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"anoted/internal/config"
)

// Segment is one transcribed span of audio, in seconds from the start.
type Segment struct {
	Start float64 `json:"start"`
	End   float64 `json:"end"`
	Text  string  `json:"text"`
}

// writeSegmentFiles renders the requested output formats from segments.
//
// The openai-whisper and whisper.cpp backends let their CLI write these files;
// faster-whisper is a library that only yields segments, so anoted renders them
// itself. Keeping this pure (segments in, files out) is also what makes the
// formats testable without invoking a model.
func writeSegmentFiles(dir, fileBase string, segs []Segment, formats []string, language string) ([]string, error) {
	want := effectiveWhisperFormats(formats)
	var written []string
	for _, f := range want {
		var (
			body []byte
			ext  string
		)
		switch f {
		case config.OutputFormatTXT:
			body, ext = []byte(RenderTXT(segs)), ".txt"
		case config.OutputFormatSRT:
			body, ext = []byte(RenderSRT(segs)), ".srt"
		case config.OutputFormatVTT:
			body, ext = []byte(RenderVTT(segs)), ".vtt"
		case config.OutputFormatJSON:
			b, err := RenderJSON(segs, language)
			if err != nil {
				return written, fmt.Errorf("render json: %w", err)
			}
			body, ext = b, ".json"
		default:
			continue
		}
		path := filepath.Join(dir, fileBase+ext)
		if err := os.WriteFile(path, body, 0o644); err != nil {
			return written, fmt.Errorf("write %s: %w", path, err)
		}
		written = append(written, path)
	}
	return written, nil
}

// RenderTXT joins segment text one line per segment.
func RenderTXT(segs []Segment) string {
	var b strings.Builder
	for _, s := range segs {
		text := strings.TrimSpace(s.Text)
		if text == "" {
			continue
		}
		b.WriteString(text)
		b.WriteByte('\n')
	}
	return b.String()
}

// RenderSRT renders SubRip subtitles. Cues are 1-indexed and use a comma as the
// millisecond separator, which is what distinguishes SRT from VTT.
func RenderSRT(segs []Segment) string {
	var b strings.Builder
	idx := 0
	for _, s := range segs {
		text := strings.TrimSpace(s.Text)
		if text == "" {
			continue
		}
		idx++
		fmt.Fprintf(&b, "%d\n", idx)
		fmt.Fprintf(&b, "%s --> %s\n", formatTimestamp(s.Start, ','), formatTimestamp(s.End, ','))
		b.WriteString(text)
		b.WriteString("\n\n")
	}
	return b.String()
}

// RenderVTT renders WebVTT subtitles.
func RenderVTT(segs []Segment) string {
	var b strings.Builder
	b.WriteString("WEBVTT\n\n")
	for _, s := range segs {
		text := strings.TrimSpace(s.Text)
		if text == "" {
			continue
		}
		fmt.Fprintf(&b, "%s --> %s\n", formatTimestamp(s.Start, '.'), formatTimestamp(s.End, '.'))
		b.WriteString(text)
		b.WriteString("\n\n")
	}
	return b.String()
}

type jsonTranscript struct {
	Text     string        `json:"text"`
	Segments []jsonSegment `json:"segments"`
	Language string        `json:"language"`
}

type jsonSegment struct {
	ID    int     `json:"id"`
	Start float64 `json:"start"`
	End   float64 `json:"end"`
	Text  string  `json:"text"`
}

// RenderJSON renders a transcript in the shape openai-whisper's JSON writer
// produces, so downstream consumers do not have to care which backend ran.
func RenderJSON(segs []Segment, language string) ([]byte, error) {
	out := jsonTranscript{Language: language, Segments: []jsonSegment{}}
	var text strings.Builder
	for i, s := range segs {
		out.Segments = append(out.Segments, jsonSegment{
			ID:    i,
			Start: s.Start,
			End:   s.End,
			Text:  s.Text,
		})
		text.WriteString(s.Text)
	}
	out.Text = strings.TrimSpace(text.String())
	return json.MarshalIndent(out, "", "  ")
}

// formatTimestamp renders seconds as HH:MM:SS<sep>mmm.
func formatTimestamp(sec float64, msSep byte) string {
	if sec < 0 {
		sec = 0
	}
	total := int64(sec*1000 + 0.5)
	ms := total % 1000
	total /= 1000
	s := total % 60
	total /= 60
	m := total % 60
	h := total / 60
	return fmt.Sprintf("%02d:%02d:%02d%c%03d", h, m, s, msSep, ms)
}
