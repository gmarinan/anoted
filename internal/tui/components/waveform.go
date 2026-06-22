package components

import (
	"math"
	"strings"

	"charm.land/lipgloss/v2"
)

const (
	eqRows        = 8
	waveNoiseGate = 0.02
)

// WaveformViz renders fixed-column spectrum equalizers for system output and mic.
type WaveformViz struct {
	SystemBands    []float64
	MicBands       []float64
	SystemLabel    string
	MicLabel       string
	Recording      bool
	LevelEnabled   bool
	LevelAvailable bool
	MonitorWarn    string
	Width          int
	LevelFrame     uint64 // changes every UI tick; keeps Bubble Tea from skipping identical frames
}

func (v WaveformViz) Render() string {
	if !v.LevelEnabled {
		return v.renderDisabled()
	}
	var b strings.Builder
	// Zero-width chars bust Bubble Tea viewEquals without visible fake motion.
	if v.LevelFrame > 0 {
		b.WriteString("\u200b")
		for i := uint64(0); i < v.LevelFrame%4; i++ {
			b.WriteString("\u200b")
		}
	}
	b.WriteString(labelStyle.Render("System audio"))
	b.WriteString("\n")
	b.WriteString(subtleStyle.Render(truncate(v.SystemLabel, v.barWidth())))
	b.WriteString("\n")
	b.WriteString(v.renderMeter(v.SystemBands, systemEQColors))
	b.WriteString("\n\n")
	b.WriteString(labelStyle.Render("Microphone"))
	b.WriteString("\n")
	b.WriteString(subtleStyle.Render(truncate(v.MicLabel, v.barWidth())))
	b.WriteString("\n")
	if v.Recording {
		b.WriteString(v.renderMeter(v.MicBands, micEQColors))
	} else {
		b.WriteString(v.renderIdleMeter())
		b.WriteString("\n")
		b.WriteString(subtleStyle.Render("— activo al grabar"))
	}
	if v.MonitorWarn != "" {
		b.WriteString("\n")
		b.WriteString(warnStyle.Render("⚠ " + v.MonitorWarn))
	}
	if !v.LevelAvailable {
		b.WriteString("\n")
		b.WriteString(subtleStyle.Render("niveles no disponibles en esta plataforma"))
	}
	return b.String()
}

func (v WaveformViz) renderDisabled() string {
	var b strings.Builder
	b.WriteString(labelStyle.Render("System audio"))
	b.WriteString("\n")
	b.WriteString(subtleStyle.Render(truncate(v.SystemLabel, v.barWidth())))
	b.WriteString("\n\n")
	b.WriteString(labelStyle.Render("Microphone"))
	b.WriteString("\n")
	b.WriteString(subtleStyle.Render(truncate(v.MicLabel, v.barWidth())))
	if v.MonitorWarn != "" {
		b.WriteString("\n")
		b.WriteString(warnStyle.Render("⚠ " + v.MonitorWarn))
	}
	return b.String()
}

func (v WaveformViz) barWidth() int {
	w := v.Width - 4
	if w < 16 {
		return 16
	}
	if w > 32 {
		return 32
	}
	return w
}

func (v WaveformViz) renderIdleMeter() string {
	return renderEqualizer(make([]float64, v.barWidth()), systemEQColors)
}

var (
	systemEQColors = [eqRows]string{"99", "99", "135", "135", "212", "212", "214", "228"}
	micEQColors    = [eqRows]string{"168", "168", "170", "170", "212", "212", "218", "225"}
	partialRunes   = []rune{'░', '▁', '▂', '▃', '▄', '▅', '▆', '▇'}
)

func (v WaveformViz) renderMeter(bands []float64, colors [eqRows]string) string {
	width := v.barWidth()
	cols := fitBands(bands, width)

	heights := make([]float64, width)
	for i, lv := range cols {
		heights[i] = bandHeight(lv, v.LevelFrame, i)
	}
	return renderEqualizer(heights, colors)
}

func bandHeight(raw float64, frame uint64, col int) float64 {
	if raw <= waveNoiseGate {
		return 0
	}
	norm := math.Pow(raw, 0.65)
	h := norm * float64(eqRows)
	if h > float64(eqRows) {
		h = float64(eqRows)
	}
	return h
}

func fitBands(bands []float64, width int) []float64 {
	out := make([]float64, width)
	if len(bands) == 0 {
		return out
	}
	for i := 0; i < width; i++ {
		idx := i * len(bands) / width
		if idx >= len(bands) {
			idx = len(bands) - 1
		}
		out[i] = bands[idx]
	}
	return out
}

func renderEqualizer(heights []float64, colors [eqRows]string) string {
	width := len(heights)
	if width == 0 {
		return ""
	}

	var lines []string
	for row := eqRows - 1; row >= 0; row-- {
		var rowChars strings.Builder
		rowChars.Grow(width * 2)
		for col := 0; col < width; col++ {
			if col > 0 {
				rowChars.WriteByte(' ')
			}
			rowChars.WriteRune(cellAt(heights[col], row))
		}
		style := lipgloss.NewStyle().Foreground(lipgloss.Color(colors[row]))
		lines = append(lines, style.Render(rowChars.String()))
	}
	return strings.Join(lines, "\n")
}

func cellAt(height float64, row int) rune {
	fill := height - float64(row)
	switch {
	case fill >= 1:
		return '█'
	case fill <= 0:
		return '░'
	default:
		idx := int(fill * float64(len(partialRunes)-1))
		if idx < 0 {
			idx = 0
		}
		if idx >= len(partialRunes) {
			idx = len(partialRunes) - 1
		}
		return partialRunes[idx]
	}
}

// scaleLevel maps linear peak amplitude to a perceptual 0..1 range using dB.
func scaleLevel(v float64) float64 {
	if v <= 1e-6 {
		return 0
	}
	db := 20 * math.Log10(v)
	const minDB = -42.0
	if db < minDB {
		db = minDB
	}
	if db > 0 {
		db = 0
	}
	return (db - minDB) / (-minDB)
}
