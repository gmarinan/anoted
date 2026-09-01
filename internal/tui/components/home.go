package components

import (
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
	// SessionsBlock and StatusBlock, when set, are the pre-rendered (typically
	// memoized) outputs of RenderMainContent / StatusBox; empty means render
	// them here.
	SessionsBlock string
	StatusBlock   string
}

// StatusPanelWidth returns the width the Home status box renders at for the
// given terminal width — the memoizing caller must match View's layout branch.
func StatusPanelWidth(width int) int {
	layout := NewPanelLayout(width)
	if layout.Width >= HomeTopRowMinWidth {
		return layout.ColumnWidth()
	}
	return layout.FullWidth()
}

func (v HomeView) View() string {
	layout := NewPanelLayout(v.Width)
	fullW := layout.FullWidth()

	sess := v.Sessions
	sess.Height = v.Height
	sess.Width = fullW
	sessionsBlock := v.SessionsBlock
	if sessionsBlock == "" {
		sessionsBlock = sess.RenderMainContent()
	}

	status := v.StatusBlock
	if status == "" {
		status = v.StatusBox(StatusPanelWidth(v.Width))
	}
	var topRow string
	if layout.Width >= HomeTopRowMinWidth {
		topRow = layout.JoinColumns(status, v.audioBox(layout.ColumnWidth(), true))
	} else {
		topRow = JoinBlocksVertical(status, v.audioBox(fullW, layout.Width < SessionsCompactWidth))
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

// StatusBox renders the Home status panel; exported so the TUI can memoize it
// (its inputs change at 1Hz while the meter repaints at up to 30Hz).
func (v HomeView) StatusBox(width int) string {
	var lines []string
	if v.Recording {
		lines = append(lines, recStyle.Render(" ● RECORDING "))
	}
	lines = append(lines, row("State", displayState(v.AppState)))
	lines = append(lines, row("Meeting", v.Provider))
	auto := okStyle.Render("on")
	if !v.AutoRecord {
		auto = subtleStyle.Render("off")
	}
	lines = append(lines, labelStyle.Render("Auto-record:")+" "+auto)
	if v.Recording {
		lines = append(lines, row("Duration", v.Duration.Round(time.Second).String()))
		// "Output: " is 8 cells and the box content area is width-4.
		lines = append(lines, row("Output", truncate(v.SessionDir, width-12)))
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
