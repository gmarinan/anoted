package tui

import (
	"anoted/internal/tui/components"
	tea "charm.land/bubbletea/v2"
)

func (m Model) quitGuarded() bool {
	return m.recording || m.transcribeActive || m.whisperInstall.Active || m.gpuInstall.Active || m.setupWizard.Busy
}

func (m Model) quitConfirmReasons() []string {
	return components.FormatQuitReasons(
		m.recording,
		m.transcribeActive,
		m.whisperInstall.Active || m.gpuInstall.Active || m.setupWizard.Busy,
	)
}

func (m Model) openQuitConfirm() Model {
	m.quitConfirmOpen = true
	m.quitConfirmCursor = 0
	return m
}

func (m Model) closeQuitConfirm() Model {
	m.quitConfirmOpen = false
	m.quitConfirmCursor = 0
	return m
}

func (m Model) handleQuitConfirmKey(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "esc", "n":
		return m.closeQuitConfirm(), nil
	case "up", "k":
		if m.quitConfirmCursor > 0 {
			m.quitConfirmCursor--
		}
		return m, nil
	case "down", "j":
		if m.quitConfirmCursor < 1 {
			m.quitConfirmCursor++
		}
		return m, nil
	case "enter", " ":
		if m.quitConfirmCursor == 0 {
			return m.closeQuitConfirm(), nil
		}
		return m.performQuit()
	case "y":
		return m.performQuit()
	}
	return m, nil
}

func (m Model) requestQuit() (tea.Model, tea.Cmd) {
	if m.quitGuarded() {
		return m.openQuitConfirm(), nil
	}
	return m.performQuit()
}

func (m Model) performQuit() (tea.Model, tea.Cmd) {
	m.quitConfirmOpen = false
	m.quitting = true
	m.levelGen++
	m = m.cancelTranscribeOnQuit()
	if m.setupCancel != nil {
		m.setupCancel()
	}
	// Whisper installs are multi-gigabyte pip downloads; quitting used to leave
	// one running with no UI attached and no way to stop it.
	m.gpuInstall.cancel()
	m.whisperInstall.cancel()
	var cmds []tea.Cmd
	if m.deps.LevelMonitor != nil {
		cmds = append(cmds, func() tea.Msg {
			_ = m.deps.LevelMonitor.Close()
			return levelStopMsg{}
		})
	}
	if m.recording {
		cmds = append(cmds, stopRecordingCmd(m, false), tea.Quit)
		return m, tea.Sequence(cmds...)
	}
	if len(cmds) > 0 {
		cmds = append(cmds, tea.Quit)
		return m, tea.Sequence(cmds...)
	}
	return m, tea.Quit
}

func (m Model) quitConfirmOverlay(base string) string {
	if !m.quitConfirmOpen {
		return base
	}
	return components.QuitConfirmView{
		Reasons: m.quitConfirmReasons(),
		Cursor:  m.quitConfirmCursor,
		Width:   m.width,
		Height:  m.height,
	}.Overlay(base)
}
