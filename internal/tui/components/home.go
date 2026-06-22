package components

import (
	"fmt"
	"strings"
	"time"
)

// HomeView renders the Home screen.
type HomeView struct {
	AppState        string
	Platform        string
	Backend         string
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

	SystemBands    []float64
	MicBands       []float64
	LevelFrame     uint64
	LevelEnabled   bool
	LevelAvailable bool
	MonitorWarn    string
}

func (v HomeView) View() string {
	layout := NewPanelLayout(v.Width)
	colW := layout.ColumnWidth()
	var b strings.Builder

	status := v.statusBox(colW)
	audio := v.audioBox(colW)
	b.WriteString(layout.JoinColumns(status, audio))
	b.WriteString("\n\n")
	b.WriteString(v.activityBox(layout.FullWidth()))
	return b.String()
}

func (v HomeView) statusBox(width int) string {
	var lines []string
	if v.Recording {
		lines = append(lines, recStyle.Render(IconRecording+" RECORDING"))
	}
	lines = append(lines, row("State", displayState(v.AppState)))
	lines = append(lines, row("Platform", v.Platform))
	lines = append(lines, row("Backend", v.Backend))
	lines = append(lines, row("Meeting", v.Provider))
	lines = append(lines, row("Auto-record", fmt.Sprintf("%v", v.AutoRecord)))
	if v.Recording {
		lines = append(lines, row("Duration", v.Duration.Round(time.Second).String()))
		lines = append(lines, row("Output", truncate(v.SessionDir, width-8)))
	}
	if v.AwaitingConfirm {
		lines = append(lines, warnStyle.Render("⚠ "+v.ConfirmPrompt))
	}
	if v.StatusNote != "" {
		lines = append(lines, okStyle.Render("✓ "+v.StatusNote))
	}
	if v.DetectionWarn != "" {
		lines = append(lines, warnStyle.Render("⚠ "+v.DetectionWarn))
	}
	if v.ErrorMsg != "" {
		lines = append(lines, errStyle.Render("✗ "+v.ErrorMsg))
	}
	return Box("Status", strings.Join(lines, "\n"), width)
}

func (v HomeView) audioBox(width int) string {
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
	}
	return Box("Audio", viz.Render(), width)
}

func (v HomeView) activityBox(width int) string {
	var msg string
	var badge string
	switch {
	case v.Recording:
		msg = IconRecording + " Recording meeting audio…"
		badge = Badge(IconRecording+" REC", "rec")
	case v.AwaitingConfirm:
		msg = "Meeting detected — confirm recording"
		badge = Badge("CONFIRM", "warn")
	case v.AppState == "in_meeting" || v.AppState == "awaiting_record_confirm":
		msg = "Meeting active — " + v.Provider
		badge = Badge("IN MEET", "meet")
	default:
		msg = "Watching for Teams / Google Meet…"
		badge = Badge("READY", "ready")
	}
	body := msg + "  " + badge
	return Box("Activity", body, width)
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

func truncate(s string, max int) string {
	if max <= 3 || len(s) <= max {
		return s
	}
	return s[:max-3] + "…"
}
