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
	// Eighth-block resolution: the boundary cell renders a partial block, so
	// the bar creeps smoothly instead of jumping one whole cell at a time.
	cells := percent / 100 * float64(barWidth)
	filled := int(cells)
	if filled > barWidth {
		filled = barWidth
	}
	bar := strings.Repeat("█", filled)
	if filled < barWidth {
		if frac := int((cells - float64(filled)) * 8); frac > 0 {
			bar += string(barPartials[frac])
		} else {
			bar += "░"
		}
		bar += strings.Repeat("░", barWidth-filled-1)
	}
	pct := int(percent)
	// An ETA of zero means "not estimable yet", not "finished" — rendering it as
	// 0m 00s at the start of a job read as though it were already done.
	etaStr := "—"
	if eta > 0 {
		etaStr = transcribe.FormatETA(eta)
	}
	// The blink drives a pulse dot rather than recoloring the whole bar; a
	// two-tone bar looked like a state change instead of a heartbeat.
	pulse := "●"
	if blink {
		pulse = "○"
	}
	body := fmt.Sprintf("%s [%s] %d%% - ETA: %s", pulse, bar, pct, etaStr)
	return txActiveStyle.Render("TX: " + body)
}

// barPartials index 1..7 maps to eighth-block fill glyphs.
var barPartials = []rune{' ', '▏', '▎', '▍', '▌', '▋', '▊', '▉'}

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

// TXStatusLabel renders a semantic TX column cell. The old "txt yes"/"aud no"
// labels leaked implementation constants into the UI; the column already means
// "transcription", so say done/none directly.
func TXStatusLabel(state string) string {
	switch state {
	case "yes":
		return txDoneStyle.Render("✓ done")
	case "err":
		return txErrorStyle.Render("err")
	case "no":
		return txPendingStyle.Render("· none")
	default:
		return txPendingStyle.Render(state)
	}
}
