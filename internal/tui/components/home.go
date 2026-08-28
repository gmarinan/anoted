package components

import (
	"fmt"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
)

// HomeView renders the Home screen (live status + session library).
type HomeView struct {
	AppState        string
	Provider        string
	SystemDevice    string
	MicDevice       string
	Recording       bool
	Duration        time.Duration
	SessionDir      string
	AutoRecord      bool
	AwaitingConfirm bool
	ConfirmPrompt   string
	StatusNote      string
	DetectionWarn   string
	ErrorMsg        string
	Width           int
	Height          int

	SystemBands    []float64
	MicBands       []float64
	LevelFrame     uint64
	LevelEnabled   bool
	LevelAvailable bool
	MonitorWarn    string

	Sessions SessionsView
}

func (v HomeView) View() string {
	layout := NewPanelLayout(v.Width)
	fullW := layout.FullWidth()

	sess := v.Sessions
	sess.Height = v.Height
	sess.Width = fullW
	sessionsBlock := sess.renderMainContent()

	var topRow string
	if layout.Width >= HomeTopRowMinWidth {
		colW := layout.ColumnWidth()
		topRow = layout.JoinColumns(v.statusBox(colW), v.audioBox(colW, true))
	} else {
		topRow = JoinBlocksVertical(v.statusBox(fullW), v.audioBox(fullW, layout.Width < SessionsCompactWidth))
	}
	content := JoinBlocksVertical(topRow, sessionsBlock)

	if sess.DeleteConfirm {
		h := v.overlayHeight(content)
		return FloatCenter(content, sess.renderDeleteModal(), v.Width, h)
	}
	if sess.OpenerPicker {
		h := v.overlayHeight(content)
		return FloatCenter(content, sess.renderOpenerModal(), v.Width, h)
	}
	return content
}

func (v HomeView) overlayHeight(base string) int {
	h := v.Height - 8
	if h < 12 {
		h = 12
	}
	baseH := lipgloss.Height(base)
	if baseH > h {
		h = baseH
	}
	return h
}

func (v HomeView) statusBox(width int) string {
	var lines []string
	if v.Recording {
		lines = append(lines, recStyle.Render(LabelRecording+" RECORDING"))
	}
	lines = append(lines, row("State", displayState(v.AppState)))
	lines = append(lines, row("Meeting", v.Provider))
	lines = append(lines, row("Auto-record", fmt.Sprintf("%v", v.AutoRecord)))
	if v.Recording {
		lines = append(lines, row("Duration", v.Duration.Round(time.Second).String()))
		lines = append(lines, row("Output", truncate(v.SessionDir, width-8)))
	}
	if v.AwaitingConfirm {
		lines = append(lines, warnStyle.Render(LabelWarn+" "+v.ConfirmPrompt))
	}
	if v.StatusNote != "" {
		lines = append(lines, okStyle.Render("✓ "+v.StatusNote))
	}
	if v.DetectionWarn != "" {
		lines = append(lines, warnStyle.Render(LabelWarn+" "+v.DetectionWarn))
	}
	if v.ErrorMsg != "" {
		lines = append(lines, errStyle.Render("✗ "+v.ErrorMsg))
	}
	return Box("Status", strings.Join(lines, "\n"), width)
}

func (v HomeView) audioBox(width int, forceCompact bool) string {
	sys := v.SystemDevice
	if sys == "" {
		sys = subtleStyle.Render("(loading…)")
	}
	mic := v.MicDevice
	if mic == "" {
		mic = subtleStyle.Render("(loading…)")
	}

	viz := WaveformViz{
		SystemBands:    v.SystemBands,
		MicBands:       v.MicBands,
		LevelFrame:     v.LevelFrame,
		SystemLabel:    sys,
		MicLabel:       mic,
		Recording:      v.Recording,
		LevelEnabled:   v.LevelEnabled,
		LevelAvailable: v.LevelAvailable,
		MonitorWarn:    v.MonitorWarn,
		Width:          width,
		ForceCompact:   forceCompact,
	}
	return Box("Audio", viz.Render(), width)
}

func displayState(state string) string {
	switch state {
	case "idle":
		return okStyle.Render("Idle")
	case "recording":
		return recStyle.Render("Recording")
	case "in_meeting":
		return magentaStyle.Render("In meeting")
	case "awaiting_record_confirm":
		return warnStyle.Render("Awaiting confirm")
	case "detecting":
		return labelStyle.Render("Detecting…")
	case "error":
		return errStyle.Render("Error")
	default:
		return valueStyle.Render(state)
	}
}

// truncate clamps s to max terminal cells.
//
// It counted runes before, which is wrong twice over: a CJK character or emoji
// occupies two cells, so a "12 rune" device name could render 24 columns wide
// and blow out its box; and reserving three runes for a one-cell ellipsis threw
// away two columns at every call site.
func truncate(s string, max int) string {
	return clampStyledWidth(s, max)
}
