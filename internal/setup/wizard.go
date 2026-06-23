package setup

import (
	"fmt"
	"strings"

	"anoted/internal/config"
	"anoted/internal/platform"
)

const (
	setupWizardSteps = 5
)

// WizardStep identifies the active setup wizard screen.
type WizardStep int

const (
	WizardWelcome WizardStep = iota
	WizardDetection
	WizardTranscription
	WizardInstalling
	WizardDone
)

// WizardState is the shared setup state for CLI and TUI.
type WizardState struct {
	Step               WizardStep
	DetCursor          int
	AutoTranscribe     bool
	InstallWhisper     bool
	TranscribeCursor   int // 0=auto, 1=install whisper
	Log                []string
	LogScroll          int
	Err                string
	Busy               bool
	DetectionLines     []string
}

// NewWizardState creates initial wizard state for the platform.
func NewWizardState(plat platform.Info) WizardState {
	choices := DetectionChoices(plat)
	cursor := 0
	for i, c := range choices {
		if c.Recommended {
			cursor = i
			break
		}
	}
	return WizardState{
		Step:           WizardWelcome,
		DetCursor:      cursor,
		AutoTranscribe: false,
		InstallWhisper: true,
	}
}

// StepTitle returns a short label for the current step.
func (w WizardState) StepTitle() string {
	switch w.Step {
	case WizardWelcome:
		return "Welcome"
	case WizardDetection:
		return "Meeting detection"
	case WizardTranscription:
		return "Transcription"
	case WizardInstalling:
		return "Installing"
	case WizardDone:
		return "Done"
	default:
		return ""
	}
}

// StepNumber returns 1-based step index for display.
func (w WizardState) StepNumber() int {
	switch w.Step {
	case WizardWelcome:
		return 1
	case WizardDetection:
		return 2
	case WizardTranscription:
		return 3
	case WizardInstalling, WizardDone:
		return 4
	default:
		return 1
	}
}

func (w *WizardState) AppendLog(line string) {
	line = strings.TrimSpace(line)
	if line == "" {
		return
	}
	w.Log = append(w.Log, line)
	const max = 500
	if len(w.Log) > max {
		w.Log = w.Log[len(w.Log)-max:]
	}
}

// VisibleLog returns log lines for the scroll viewport.
func (w WizardState) VisibleLog(height int) []string {
	if height < 1 {
		height = 8
	}
	if len(w.Log) <= height {
		return w.Log
	}
	start := w.LogScroll
	if start < 0 {
		start = 0
	}
	maxStart := len(w.Log) - height
	if start > maxStart {
		start = maxStart
	}
	return w.Log[start : start+height]
}

// MaxLogScroll returns the maximum scroll offset.
func (w WizardState) MaxLogScroll(viewHeight int) int {
	if viewHeight < 1 {
		viewHeight = 8
	}
	n := len(w.Log) - viewHeight
	if n < 0 {
		return 0
	}
	return n
}

// SelectedDetectionMode returns the chosen detection mode.
func (w WizardState) SelectedDetectionMode(plat platform.Info) string {
	choices := DetectionChoices(plat)
	if w.DetCursor < 0 || w.DetCursor >= len(choices) {
		return DefaultDetectionMode(plat)
	}
	return choices[w.DetCursor].Mode
}

// SummaryLines returns final setup summary lines.
func SummaryLines(cfg config.Config) []string {
	var lines []string
	lines = append(lines, fmt.Sprintf("detection.mode: %s", cfg.Detection.Mode))
	if cfg.Transcription.AutoAfterRecording {
		lines = append(lines, "transcription.auto_after_recording: true")
	}
	if cfg.Transcription.Binary != "" {
		lines = append(lines, "whisper: "+cfg.Transcription.Binary)
	}
	return lines
}
