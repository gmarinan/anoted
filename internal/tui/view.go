package tui

import (
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"meetctl/internal/tui/components"
)

func (m Model) View() tea.View {
	var content strings.Builder

	switch m.screen {
	case ScreenAudio:
		content.WriteString(m.audioView().View())
	case ScreenDoctor:
		content.WriteString(m.doctorView().View())
	case ScreenSessions:
		content.WriteString(m.sessionsView().View())
	case ScreenConfig:
		content.WriteString(m.configView().View())
	default:
		content.WriteString(m.homeView().View())
	}

	content.WriteString("\n\n")
	content.WriteString(components.FooterForTab(components.ScreenToTab(string(m.screen)), m.awaitingRecordConfirm))

	v := tea.NewView(content.String())
	v.AltScreen = true
	v.WindowTitle = "meetctl"
	if m.deps.Config.Privacy.ShowRecordingIndicator && m.recording {
		v.WindowTitle = "meetctl ● RECORDING"
	}
	return v
}

func (m Model) homeView() components.HomeView {
	duration := time.Duration(0)
	if m.recording && !m.recordStart.IsZero() {
		duration = time.Since(m.recordStart)
	}
	backend := m.deps.Recorder.Name()
	if m.recStatus.Backend != "" {
		backend = m.recStatus.Backend
	}
	return components.HomeView{
		AppState:        string(m.appState),
		Platform:        m.deps.Platform.Name(),
		Backend:         backend,
		SystemDevice:    m.systemDevice,
		MicDevice:       m.micDevice,
		Provider:        displayProvider(m.provider, m.detection.Title),
		Recording:       m.recording,
		Duration:        duration,
		SessionDir:      m.sessionDir,
		AutoRecord:      m.autoRecord,
		AwaitingConfirm: m.awaitingRecordConfirm,
		ConfirmPrompt:   "Meeting detected — start recording? [y/n]",
			StatusNote:      m.statusNote,
			DetectionWarn:   m.detection.Warning,
		ErrorMsg:        m.errMsg,
		Width:           m.width,
	}
}

func (m Model) audioView() components.AudioView {
	return components.AudioView{
		Catalog:       m.audioCatalog,
		Section:       m.audioSection,
		Cursor:        m.audioCursor,
		SystemMonitor: m.deps.Config.Audio.SystemMonitor,
		Microphone:    m.deps.Config.Audio.Microphone,
		Loading:       m.audioLoading,
		ErrMsg:        m.audioErr,
		SavedMsg:      m.audioSaved,
		MonitorWarn:   m.audioMonitorWarn,
		Width:         m.width,
	}
}

func (m Model) doctorView() components.DoctorView {
	backend := m.deps.Recorder.Name()
	if m.recStatus.Backend != "" {
		backend = m.recStatus.Backend
	}
	return components.DoctorView{
		Report:        m.doctorReport,
		AppState:      string(m.appState),
		Platform:      m.deps.Platform.Name(),
		Backend:       backend,
		Provider:      displayProvider(m.provider, m.detection.Title),
		SystemDevice:  m.systemDevice,
		MicDevice:     m.micDevice,
		DetectionWarn: m.detection.Warning,
		Width:         m.width,
	}
}

func (m Model) sessionsView() components.SessionsView {
	return components.SessionsView{
		Records:      m.sessions,
		Cursor:       m.sessionCursor,
		ErrMsg:       m.sessionsErr,
		Transcribing: m.transcribing,
		StatusNote:   m.transcribeNote,
		Width:        m.width,
	}
}

func (m Model) configView() components.ConfigView {
	return components.ConfigView{
		Path:      m.deps.ConfigPath,
		Lines:     m.configLines,
		CursorRow: m.configCursorRow,
		CursorCol: m.configCursorCol,
		ScrollRow: m.configScrollRow,
		Dirty:     m.configDirty,
		ErrMsg:    m.configErr,
		SavedMsg:  m.configSavedMsg,
		Width:     m.width,
		Height:    m.height,
	}
}

func displayProvider(provider, title string) string {
	if provider != "" && provider != "none" {
		switch provider {
		case "google_meet":
			return "Google Meet"
		case "teams":
			return "Microsoft Teams"
		default:
			return provider
		}
	}
	if title != "" {
		return title
	}
	return "None detected"
}
