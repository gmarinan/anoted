package tui

import (
	"anoted/internal/config"
	"anoted/internal/setup"
	"anoted/internal/transcribe"
	tea "charm.land/bubbletea/v2"
)

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
