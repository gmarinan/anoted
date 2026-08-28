package tui

import (
	"time"

	"anoted/internal/tui/components"
	tea "charm.land/bubbletea/v2"
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
		cmds = append(cmds, doctorReportCmd(m.deps.Config), refreshDoctorCapsCmd(m.deps.Config))
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

	if m.setupOpen {
		if key == "q" || key == "ctrl+c" {
			return m.requestQuit()
		}
		return m.handleSetupKey(key)
	}

	if m.quitConfirmOpen {
		return m.handleQuitConfirmKey(key)
	}

	// Config text fields must see "q" before the global quit binding does.
	// Otherwise typing a path like ~/Documents/quarterly, the language code "qu"
	// or any detection pattern containing a q quit the app outright and threw
	// away the edit. ctrl+c stays global as the escape hatch.
	if m.screen == ScreenConfig && m.configAbsorbsKeys() && key != "ctrl+c" {
		return m.handleConfigKey(msg)
	}

	// The help overlay swallows everything while open so it behaves like the
	// other modals.
	if m.helpOpen {
		switch key {
		case "?", "esc", "q", "enter":
			m.helpOpen = false
		case "ctrl+c":
			return m.requestQuit()
		}
		return m, nil
	}

	switch key {
	case "q", "ctrl+c":
		return m.requestQuit()
	}

	// Home's own overlays own esc, so only claim it when none is open.
	if !m.homeAbsorbsKeys() {
		switch key {
		case "?":
			m.helpOpen = true
			return m, nil
		case "esc":
			// Esc was a dead key at the top level, though "go back" is what
			// every terminal user expects it to do.
			if m.screen != ScreenMain {
				return m.switchTab(components.TabHome)
			}
			m.errMsg = ""
			m.sessionsErr = ""
			m.statusNote = ""
			m.sessionsDesktopNote = ""
			return m, nil
		}
	}

	if m.screen == ScreenConfig {
		if m.isTabSwitchKey(key) {
			model, cmd, _ := m.handleTabSwitch(key)
			return model, cmd
		}
		return m.handleConfigKey(msg)
	}

	// Dismiss Home overlays before leaving, not after: handleTabSwitch already
	// returns the next model, so clearing the flags on the old receiver here was
	// discarded and the delete-confirm modal reappeared on the way back.
	if m.screen == ScreenMain && m.isTabSwitchKey(key) {
		m.sessionsOpenerPicker = false
		m.sessionsDeleteConfirm = false
	}
	if model, cmd, ok := m.handleTabSwitch(key); ok {
		return model, cmd
	}

	switch m.screen {
	case ScreenMain:
		return m.handleHomeKey(msg)
	}

	switch key {
	case "R":
		return m.handleRefresh()
	case "i":
		if m.whisperCanInstall() {
			return m.startWhisperInstall()
		}
	case "g":
		if m.doctorGPUOffer() {
			return m.startGPUInstall()
		}
	case "pgup":
		if m.gpuInstallActive || len(m.gpuInstallLog) > 0 {
			m.gpuInstallScroll = clampScroll(m.gpuInstallScroll-3, m.maxGPUInstallScroll(installLogRows))
			return m, nil
		}
		if m.whisperInstallActive || len(m.whisperInstallLog) > 0 {
			m.whisperInstallScroll = clampScroll(m.whisperInstallScroll-3, m.maxWhisperInstallScroll(installLogRows))
			return m, nil
		}
	case "pgdown":
		if m.gpuInstallActive || len(m.gpuInstallLog) > 0 {
			m.gpuInstallScroll = clampScroll(m.gpuInstallScroll+3, m.maxGPUInstallScroll(installLogRows))
			return m, nil
		}
		if m.whisperInstallActive || len(m.whisperInstallLog) > 0 {
			m.whisperInstallScroll = clampScroll(m.whisperInstallScroll+3, m.maxWhisperInstallScroll(installLogRows))
			return m, nil
		}
	case "S":
		return m.openSetupWizard(), nil
	}
	return m, nil
}

// homeAbsorbsKeys reports whether a Home overlay is claiming keys, mirroring
// configAbsorbsKeys. Without it the global esc binding stole the key from the
// delete-confirm and opener modals, which use it to cancel.
func (m Model) homeAbsorbsKeys() bool {
	if m.screen != ScreenMain {
		return false
	}
	// awaitingRecordConfirm is here because esc dismisses the "start recording?"
	// prompt, which is the one place esc already did something on Home.
	return m.sessionsDeleteConfirm || m.sessionsOpenerPicker || m.awaitingRecordConfirm
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
			m.recordOpAt = time.Now()
			return m, stopRecordingCmd(m, false)
		}
		// Refuse up front rather than starting a backend that cannot capture.
		if reason := m.recorderUnusable(); reason != "" {
			m.errMsg = reason
			return m, nil
		}
		// Two quick presses used to fire two concurrent starts, the second of
		// which lost the mutex race and reported "already recording".
		if m.recordOpInFlight {
			return m, nil
		}
		m.autoRecordFailures = 0
		m.recordOpInFlight = true
		m.recordOpAt = time.Now()
		return m, startRecordingCmd(m)
	case "a":
		m.autoRecord = !m.autoRecord
		if !m.autoRecord {
			m.awaitingRecordConfirm = false
			m.wantAutoRecordResume = false
			m.resumeForSessionKey = ""
			m.autoRecordRetryAfter = time.Time{}
			m.autoRecordFailures = 0
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
			m.recordOpAt = time.Now()
			return m, startRecordingCmd(m)
		}
		return m, nil
	case "y", "enter":
		if m.awaitingRecordConfirm && !m.recording {
			m.awaitingRecordConfirm = false
			m.recordOpInFlight = true
			m.recordOpAt = time.Now()
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
		return m, tea.Batch(doctorReportCmd(m.deps.Config), refreshDoctorCapsCmd(m.deps.Config))
	case ScreenMain:
		m = m.refreshSessions()
		return m, tea.Batch(resolveDeviceLabelsCmd(m), m.startSystemLevelCmd())
	case ScreenConfig:
		return m.reloadConfigFromDisk(), resolveDeviceLabelsCmd(m)
	default:
		return m, resolveDeviceLabelsCmd(m)
	}
}

// installLogRows is the visible height of the Doctor install log panes. Both
// panes use the same window so their scroll maths agree with the view.
const installLogRows = 8

func clampScroll(v, max int) int {
	if v < 0 {
		return 0
	}
	if v > max {
		return max
	}
	return v
}
