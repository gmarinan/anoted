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
	// An ETA of zero means "not estimable yet", not "finished" — rendering it as
	// 0m 00s at the start of a job read as though it were already done.
	etaStr := "—"
	if eta > 0 {
		etaStr = transcribe.FormatETA(eta)
	}
	body := fmt.Sprintf("[%s] %d%% - ETA: %s", bar, pct, etaStr)
	style := txActiveStyle
	if blink {
		style = txActiveAltStyle
	}
	return style.Render("TX: " + body)
}

// TranscribeProgressCompact is a short TX cell for narrow session tables.
func TranscribeProgressCompact(percent float64, blink bool) string {
	if percent < 0 {
		percent = 0
	}
	if percent > 100 {
		percent = 100
	}
	body := fmt.Sprintf("%d%%", int(percent))
	style := txActiveStyle
	if blink {
		style = txActiveAltStyle
	}
	return style.Render(body)
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
