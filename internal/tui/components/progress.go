package components

import (
	"fmt"
	"strings"
	"time"

	"anoted/internal/transcribe"
)

// TranscribeProgressBar renders TX: [████░░░░] 65% - ETA: 0m 15s
func TranscribeProgressBar(percent float64, barWidth int, eta time.Duration, blink bool) string {
	if barWidth < 8 {
		barWidth = 8
	}
	if percent < 0 {
		percent = 0
	}
	if percent > 100 {
		percent = 100
	}
	filled := int(percent / 100 * float64(barWidth))
	if filled > barWidth {
		filled = barWidth
	}
	bar := strings.Repeat("█", filled) + strings.Repeat("░", barWidth-filled)
	pct := int(percent)
	etaStr := transcribe.FormatETA(eta)
	body := fmt.Sprintf("[%s] %d%% - ETA: %s", bar, pct, etaStr)
	style := txActiveStyle
	if blink {
		style = txActiveAltStyle
	}
	return style.Render("TX: " + body)
}

// TXStatusLabel renders a semantic TX column cell.
func TXStatusLabel(state string) string {
	switch state {
	case "yes":
		return txDoneStyle.Render(LabelTranscribed + " yes")
	case "err":
		return txErrorStyle.Render("err")
	case "no":
		return txPendingStyle.Render(LabelAudioSaved + " no")
	default:
		return txPendingStyle.Render(state)
	}
}
