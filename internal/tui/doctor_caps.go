package tui

import (
	"anoted/internal/config"
	"anoted/internal/doctor"
	"anoted/internal/setup"
	"anoted/internal/transcribe"
	tea "charm.land/bubbletea/v2"
)

// doctorReportMsg carries a completed doctor run back to the Update loop.
type doctorReportMsg struct {
	report doctor.Report
}

// doctorReportCmd runs the dependency checks off the Bubble Tea goroutine.
//
// doctor.Run enumerates audio devices, does a PATH sweep and spawns pactl (on
// Windows, COM enumeration plus two Python interpreters), measured here at
// ~170ms. Running it inline in Update froze the UI for that long on every
// screen switch and refresh, delaying keypresses, the duration tick, and a
// pending meeting-end auto-stop by the same amount.
func doctorReportCmd(cfg config.Config) tea.Cmd {
	return func() tea.Msg {
		return doctorReportMsg{report: doctor.Run(cfg)}
	}
}

// doctorCapsMsg carries cached Doctor-tab install availability (expensive checks).
type doctorCapsMsg struct {
	whisperCanInstall bool
	gpuCanInstall     bool
}

func refreshDoctorCapsCmd(cfg config.Config) tea.Cmd {
	return func() tea.Msg {
		return doctorCapsMsg{
			whisperCanInstall: !transcribe.IsInstalled(cfg),
			gpuCanInstall:     setup.GPUOfferAvailable(cfg),
		}
	}
}

func (m Model) handleDoctorCaps(msg doctorCapsMsg) Model {
	m.doctorWhisperCanInstall = msg.whisperCanInstall
	m.doctorGPUCanInstall = msg.gpuCanInstall
	return m
}

func (m Model) invalidateDoctorCapsCmd() tea.Cmd {
	return refreshDoctorCapsCmd(m.deps.Config)
}

// doctorWhisperOffer reports cached whisper install availability for UI hints.
func (m Model) doctorWhisperOffer() bool {
	return m.doctorWhisperCanInstall && !m.whisperInstallActive
}

// doctorGPUOffer reports cached GPU install availability for UI hints.
func (m Model) doctorGPUOffer() bool {
	return m.doctorGPUCanInstall && !m.gpuInstallActive
}
