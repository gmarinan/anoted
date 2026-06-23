package tui

import (
	tea "charm.land/bubbletea/v2"
	"anoted/internal/tui/components"
)

func (m Model) switchScreen(screen Screen) (tea.Model, tea.Cmd) {
	leavingMain := m.screen == ScreenMain && screen != ScreenMain
	enteringMain := m.screen != ScreenMain && screen == ScreenMain

	if leavingMain {
		sessionScroll.reset()
	}

	var cmds []tea.Cmd
	if leavingMain {
		var leaveCmds []tea.Cmd
		m, leaveCmds = m.leaveHomeLevels()
		cmds = append(cmds, leaveCmds...)
	}

	m.screen = screen

	switch screen {
	case ScreenDoctor:
		m.doctorReport = loadDoctorReport(m.deps.Config)
	case ScreenMain:
		m = m.refreshSessions()
		m.audioMonitorWarn = m.deps.Audio.MonitorWarning(m.deps.Config.Audio.SystemMonitor)
		cmds = append(cmds, resolveDeviceLabelsCmd(m))
		if enteringMain {
			var enterCmds []tea.Cmd
			m, enterCmds = m.enterHomeLevels()
			cmds = append(cmds, enterCmds...)
		}
	case ScreenConfig:
		m = m.initConfigMenu()
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
		m.levelGen++
		m = m.cancelTranscribeOnQuit()
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

	if m.screen == ScreenConfig {
		if !m.configAbsorbsKeys() && m.isTabSwitchKey(key) {
			model, cmd, _ := m.handleTabSwitch(key)
			return model, cmd
		}
		return m.handleConfigKey(msg)
	}

	if model, cmd, ok := m.handleTabSwitch(key); ok {
		if m.screen == ScreenMain {
			m.sessionsOpenerPicker = false
			m.sessionsDeleteConfirm = false
		}
		return model, cmd
	}

	switch m.screen {
	case ScreenMain:
		return m.handleHomeKey(msg)
	}

	switch key {
	case "R":
		return m.handleRefresh()
	}
	return m, nil
}

func (m Model) handleHomeKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	if m.sessionsDeleteConfirm || m.sessionsOpenerPicker {
		return m.handleSessionsKey(msg)
	}

	switch key {
	case "up", "down", "k", "j", "[", "]", "pgup", "pgdown", "t", "o", "p", "f", "d", "delete":
		return m.handleSessionsKey(msg)
	case "s":
		if m.transcribeActive {
			return m.handleSessionsKey(msg)
		}
	}

	switch key {
	case "r":
		if m.recording {
			m.recordOpInFlight = true
			return m, stopRecordingCmd(m, false)
		}
		m.recordOpInFlight = true
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
			m.recordOpInFlight = true
			return m, startRecordingCmd(m)
		}
		return m, nil
	case "y", "enter":
		if m.awaitingRecordConfirm && !m.recording {
			m.awaitingRecordConfirm = false
			m.recordOpInFlight = true
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
	case "2":
		model, cmd := m.switchTab(components.TabDoctor)
		return model, cmd, true
	case "3":
		model, cmd := m.switchTab(components.TabConfig)
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
	case ScreenMain:
		m = m.refreshSessions()
		return m, tea.Batch(resolveDeviceLabelsCmd(m), m.startSystemLevelCmd())
	case ScreenConfig:
		return m.reloadConfigFromDisk(), resolveDeviceLabelsCmd(m)
	default:
		return m, resolveDeviceLabelsCmd(m)
	}
}
