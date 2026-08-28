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
	LevelFrame     uint64 // UI tick counter; Render must not depend on it (see waveform_test.go)
	ForceCompact   bool   // narrow column layout (e.g. status | audio side-by-side)
}

func (v WaveformViz) Render() string {
	if !v.LevelEnabled {
		return v.renderDisabled()
	}
	if v.ultraCompact() {
		return v.renderUltraCompact()
	}
	if v.compact() {
		return v.renderCompact()
	}
	var b strings.Builder
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
		b.WriteString(subtleStyle.Render("— active while recording"))
	}
	if v.MonitorWarn != "" {
		b.WriteString("\n")
		b.WriteString(warnStyle.Render(LabelWarn + " " + v.MonitorWarn))
	}
	if !v.LevelAvailable {
		b.WriteString("\n")
		b.WriteString(subtleStyle.Render("levels unavailable on this platform"))
	}
	return b.String()
}

func (v WaveformViz) renderDisabled() string {
	if v.compact() {
		var b strings.Builder
		b.WriteString(labelStyle.Render("System"))
		b.WriteString(" ")
		b.WriteString(subtleStyle.Render(truncate(v.SystemLabel, max(8, v.Width/3))))
		b.WriteString("\n")
		b.WriteString(labelStyle.Render("Mic"))
		b.WriteString(" ")
		b.WriteString(subtleStyle.Render(truncate(v.MicLabel, max(8, v.Width/3))))
		if v.MonitorWarn != "" {
			b.WriteString("\n")
			b.WriteString(warnStyle.Render(LabelWarn + " " + truncate(v.MonitorWarn, v.Width-4)))
		}
		return b.String()
	}
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
		b.WriteString(warnStyle.Render(LabelWarn + " " + v.MonitorWarn))
	}
	return b.String()
}

func (v WaveformViz) barWidth() int {
	w := v.Width - 4
	if v.ultraCompact() {
		if w < 6 {
			return 6
		}
		if w > 10 {
			return 10
		}
		return w
	}
	if v.compact() {
		if w < 8 {
			return 8
		}
		if w > 12 {
			return 12
		}
		return w
	}
	if w < 16 {
		return 16
	}
	if w > 32 {
		return 32
	}
	return w
}

func (v WaveformViz) compact() bool {
	return v.ForceCompact || v.Width < WaveformCompactWidth
}

func (v WaveformViz) ultraCompact() bool {
	return v.compact() && v.Width < 48
}

func (v WaveformViz) eqRowCount() int {
	if v.ultraCompact() {
		return 1
	}
	if v.compact() {
		return 3
	}
	return eqRows
}

func (v WaveformViz) renderUltraCompact() string {
	var b strings.Builder
	b.WriteString(labelStyle.Render("Sys"))
	b.WriteString(subtleStyle.Render(" " + truncate(v.SystemLabel, max(6, v.Width/4))))
	b.WriteString("\n")
	b.WriteString(v.renderSparkline(v.SystemBands, systemEQColors))
	b.WriteString("\n")
	b.WriteString(labelStyle.Render("Mic"))
	if v.Recording {
		b.WriteString("\n")
		b.WriteString(v.renderSparkline(v.MicBands, micEQColors))
	} else {
		b.WriteString(subtleStyle.Render(" —"))
	}
	return b.String()
}

func (v WaveformViz) renderSparkline(bands []float64, colors [eqRows]string) string {
	n := v.barWidth()
	cols := fitBands(bands, n)
	var chars strings.Builder
	chars.Grow(n)
	for i, lv := range cols {
		h := bandHeight(lv, v.LevelFrame, i)
		idx := int(h * float64(len(partialRunes)-1) / float64(eqRows))
		if idx < 0 {
			idx = 0
		}
		if idx >= len(partialRunes) {
			idx = len(partialRunes) - 1
		}
		chars.WriteRune(partialRunes[idx])
	}
	colorIdx := eqRows - 1
	return lipgloss.NewStyle().Foreground(lipgloss.Color(colors[colorIdx])).Render(chars.String())
}

func (v WaveformViz) renderCompact() string {
	var b strings.Builder
	b.WriteString(labelStyle.Render("System"))
	b.WriteString(" ")
	b.WriteString(subtleStyle.Render(truncate(v.SystemLabel, max(8, v.Width/3))))
	b.WriteString("\n")
	b.WriteString(v.renderMeterRows(v.SystemBands, systemEQColors))
	b.WriteString("\n")
	b.WriteString(labelStyle.Render("Mic"))
	b.WriteString(" ")
	b.WriteString(subtleStyle.Render(truncate(v.MicLabel, max(8, v.Width/3))))
	b.WriteString("\n")
	if v.Recording {
		b.WriteString(v.renderMeterRows(v.MicBands, micEQColors))
	} else {
		b.WriteString(subtleStyle.Render("— while recording"))
	}
	if v.MonitorWarn != "" {
		b.WriteString("\n")
		b.WriteString(warnStyle.Render(LabelWarn + " " + truncate(v.MonitorWarn, v.Width-4)))
	}
	return b.String()
}

func (v WaveformViz) renderMeterRows(bands []float64, colors [eqRows]string) string {
	width := v.barWidth()
	cols := fitBands(bands, width)
	rows := v.eqRowCount()
	heights := make([]float64, width)
	for i, lv := range cols {
		heights[i] = bandHeight(lv, v.LevelFrame, i)
	}
	// Scale heights to compact row count
	maxH := float64(rows)
	for i := range heights {
		heights[i] = heights[i] * maxH / float64(eqRows)
	}
	return renderEqualizerRows(heights, colors, rows, v.compact())
}

func (v WaveformViz) renderIdleMeter() string {
	return renderEqualizerRows(make([]float64, v.barWidth()), systemEQColors, v.eqRowCount(), v.compact())
}

var (
	systemEQColors = [eqRows]string{"99", "99", "135", "135", "212", "212", "214", "228"}
	micEQColors    = [eqRows]string{"168", "168", "170", "170", "212", "212", "218", "225"}
	partialRunes   = []rune{'░', '▁', '▂', '▃', '▄', '▅', '▆', '▇'}
)

func (v WaveformViz) renderMeter(bands []float64, colors [eqRows]string) string {
	return v.renderMeterRows(bands, colors)
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
	return renderEqualizerRows(heights, colors, eqRows, false)
}

func renderEqualizerRows(heights []float64, colors [eqRows]string, rows int, tight bool) string {
	width := len(heights)
	if width == 0 || rows <= 0 {
		return ""
	}

	var lines []string
	for row := rows - 1; row >= 0; row-- {
		fullRow := row
		if rows > 1 {
			fullRow = row * (eqRows - 1) / (rows - 1)
		}
		var rowChars strings.Builder
		rowChars.Grow(width * 2)
		for col := 0; col < width; col++ {
			if col > 0 && !tight {
				rowChars.WriteByte(' ')
			}
			rowChars.WriteRune(cellAt(heights[col], fullRow))
		}
		style := lipgloss.NewStyle().Foreground(lipgloss.Color(colors[fullRow]))
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
