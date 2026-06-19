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
	case ScreenDoctor:
		content.WriteString(components.DoctorPanel(m.doctorLines))
	case ScreenSessions:
		content.WriteString(components.SessionsPanel(m.sessionLines))
	case ScreenAudio:
		panel := components.AudioPanel{
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
		content.WriteString(panel.View())
	default:
		duration := time.Duration(0)
		if m.recording && !m.recordStart.IsZero() {
			duration = time.Since(m.recordStart)
		}
		backend := m.deps.Recorder.Name()
		if m.recStatus.Backend != "" {
			backend = m.recStatus.Backend
		}
		panel := components.StatusPanel{
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
		}
		content.WriteString(panel.View(m.width))
	}

	content.WriteString("\n\n")
	if m.screen == ScreenAudio {
		content.WriteString(components.AudioHelpBar())
	} else {
		content.WriteString(components.HelpBar(m.awaitingRecordConfirm))
	}

	v := tea.NewView(content.String())
	v.AltScreen = true
	v.WindowTitle = "meetctl"
	if m.deps.Config.Privacy.ShowRecordingIndicator && m.recording {
		v.WindowTitle = "meetctl ● RECORDING"
	}
	return v
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
	return "none"
}
