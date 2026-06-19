package tui

import (
	tea "charm.land/bubbletea/v2"
	"meetctl/internal/tui/components"
)

func (m Model) switchScreen(screen Screen) (tea.Model, tea.Cmd) {
	m.screen = screen
	var cmds []tea.Cmd

	switch screen {
	case ScreenDoctor:
		m.doctorReport = loadDoctorReport(m.deps.Config)
	case ScreenSessions:
		recs, err := loadSessionRecords(m.deps.Store)
		if err != nil {
			m.sessionsErr = err.Error()
			m.sessions = nil
		} else {
			m.sessionsErr = ""
			m.sessions = recs
		}
		if m.sessionCursor >= len(m.sessions) {
			m.sessionCursor = 0
		}
	case ScreenAudio:
		m.audioSection = components.AudioSectionOutput
		m.audioCursor = 0
		m.audioErr = ""
		m.audioSaved = ""
		m.audioMonitorWarn = m.deps.Audio.MonitorWarning(m.deps.Config.Audio.SystemMonitor)
		if len(m.audioCatalog.Outputs) == 0 && !m.audioLoading {
			m.audioLoading = true
			cmds = append(cmds, loadAudioCatalogCmd(m))
		}
		cmds = append(cmds, resolveDeviceLabelsCmd(m))
	case ScreenConfig:
		m = m.loadConfigEditor()
	}

	return m, tea.Batch(cmds...)
}

func (m Model) switchTab(tab components.TabID) (tea.Model, tea.Cmd) {
	return m.switchScreen(Screen(components.TabToScreen(tab)))
}

func (m Model) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	switch key {
	case "q", "ctrl+c":
		m.quitting = true
		if m.recording {
			return m, tea.Sequence(stopRecordingCmd(m, false), tea.Quit)
		}
		return m, tea.Quit
	}

	if m.screen == ScreenConfig {
		if m.isTabSwitchKey(key) {
			model, cmd, _ := m.handleTabSwitch(key)
			return model, cmd
		}
		return m.handleConfigKey(msg)
	}

	if model, cmd, ok := m.handleTabSwitch(key); ok {
		return model, cmd
	}

	switch m.screen {
	case ScreenAudio:
		return m.handleAudioKey(msg)
	case ScreenSessions:
		return m.handleSessionsKey(msg)
	}

	switch key {
	case "r":
		if m.recording {
			return m, stopRecordingCmd(m, false)
		}
		return m, startRecordingCmd(m)
	case "a":
		m.autoRecord = !m.autoRecord
		if !m.autoRecord {
			m.awaitingRecordConfirm = false
			return m, nil
		}
		if m.detection.InMeeting && !m.recording {
			if m.deps.Config.AutoRecordRequiresConfirmation {
				if !m.recordConfirmDismissed {
					m.awaitingRecordConfirm = true
					m.appState = StateAwaitingRecordConfirm
				}
				return m, nil
			}
			return m, startRecordingCmd(m)
		}
		return m, nil
	case "y", "enter":
		if m.awaitingRecordConfirm && !m.recording {
			m.awaitingRecordConfirm = false
			return m, startRecordingCmd(m)
		}
		return m, nil
	case "n", "esc":
		if m.awaitingRecordConfirm {
			m.awaitingRecordConfirm = false
			m.recordConfirmDismissed = true
			m.appState = StateInMeeting
			return m, nil
		}
		return m, nil
	case "R":
		return m.handleRefresh()
	}
	return m, nil
}

func (m Model) handleTabSwitch(key string) (tea.Model, tea.Cmd, bool) {
	switch key {
	case "1":
		model, cmd := m.switchTab(components.TabHome)
		return model, cmd, true
	case "2", "o":
		model, cmd := m.switchTab(components.TabAudio)
		return model, cmd, true
	case "3", "d":
		model, cmd := m.switchTab(components.TabDoctor)
		return model, cmd, true
	case "4":
		model, cmd := m.switchTab(components.TabSessions)
		return model, cmd, true
	case "5", "c":
		model, cmd := m.switchTab(components.TabConfig)
		return model, cmd, true
	case "tab":
		model, cmd := m.switchTab(components.NextTab(components.ScreenToTab(string(m.screen))))
		return model, cmd, true
	case "shift+tab":
		model, cmd := m.switchTab(components.PrevTab(components.ScreenToTab(string(m.screen))))
		return model, cmd, true
	default:
		return m, nil, false
	}
}

func (m Model) handleRefresh() (tea.Model, tea.Cmd) {
	switch m.screen {
	case ScreenDoctor:
		m.doctorReport = loadDoctorReport(m.deps.Config)
		return m, nil
	case ScreenSessions:
		return m.switchScreen(ScreenSessions)
	case ScreenAudio:
		m.audioLoading = true
		m.audioErr = ""
		return m, loadAudioCatalogCmd(m)
	case ScreenConfig:
		m = m.loadConfigEditor()
		return m, nil
	default:
		return m, resolveDeviceLabelsCmd(m)
	}
}
