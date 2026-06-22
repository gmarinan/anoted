package components

import (
	"strings"
)

// PreviewMode selects preview panel content.
type PreviewMode int

const (
	PreviewIdle PreviewMode = iota
	PreviewTranscript
	PreviewTranscribing
)

// PreviewPanel renders the session preview / live transcription log.
func PreviewPanel(mode PreviewMode, text string, logLines []string, width int) string {
	var body string
	switch mode {
	case PreviewTranscribing:
		if len(logLines) == 0 {
			body = subtleStyle.Render("Waiting for Whisper output…")
		} else {
			body = strings.Join(logLines, "\n")
		}
	case PreviewTranscript:
		if strings.TrimSpace(text) == "" {
			body = subtleStyle.Render("(empty transcript)")
		} else {
			body = valueStyle.Render(text)
		}
	default:
		body = subtleStyle.Render("(select a transcribed session)")
	}
	return Box("Preview", body, width)
}
