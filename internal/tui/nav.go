package tui

import (
	tea "charm.land/bubbletea/v2"
	"meetctl/internal/tui/components"
)

func (m Model) switchScreen(screen Screen) (tea.Model, tea.Cmd) {
	leavingMain := m.screen == ScreenMain && screen != ScreenMain
	enteringMain := m.screen != ScreenMain && screen == ScreenMain

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
		m.sessionsPage = 0
		if len(m.sessions) > 0 && m.sessionCursor > 0 {
			m.sessionsPage = m.sessionCursor / sessionsPageSize
			m.sessionCursor = m.sessionCursor % sessionsPageSize
		}
		m.sessionsOpenerPicker = false
		m.sessionsDeleteConfirm = false
	case ScreenMain:
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
		if m.screen == ScreenSessions {
			m.sessionsOpenerPicker = false
			m.sessionsDeleteConfirm = false
		}
		return model, cmd
	}

	switch m.screen {
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
	case "2":
		model, cmd := m.switchTab(components.TabDoctor)
		return model, cmd, true
	case "3":
		model, cmd := m.switchTab(components.TabSessions)
		return model, cmd, true
	case "4":
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
	case ScreenSessions:
		return m.switchScreen(ScreenSessions)
	case ScreenMain:
		return m, tea.Batch(resolveDeviceLabelsCmd(m), m.startSystemLevelCmd())
	case ScreenConfig:
		return m.reloadConfigFromDisk(), resolveDeviceLabelsCmd(m)
	default:
		return m, resolveDeviceLabelsCmd(m)
	}
}
