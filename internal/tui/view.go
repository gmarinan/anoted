package tui

import (
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"meetctl/internal/tui/components"
	"meetctl/internal/open"
)

func (m Model) View() tea.View {
	var content strings.Builder

	tab := components.ScreenToTab(string(m.screen))
	content.WriteString(components.Header())
	content.WriteString("\n")
	content.WriteString(components.TabBar(tab))
	content.WriteString("\n\n")

	switch m.screen {
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
	content.WriteString(components.FooterForTab(tab, m.awaitingRecordConfirm, m.sessionsFooter(), m.configFooter(), m.configSavedMsg, m.configErr, m.width))

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

		AudioCatalog:  m.audioCatalog,
		AudioSection:  m.audioSection,
		AudioCursor:   m.audioCursor,
		SystemMonitor: m.deps.Config.Audio.SystemMonitor,
		Microphone:    m.deps.Config.Audio.Microphone,
		AudioLoading:  m.audioLoading,
		AudioErr:      m.audioErr,
		AudioSaved:    m.audioSaved,
		MonitorWarn:   m.audioMonitorWarn,
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
	rec, _ := m.selectedSession()
	v := components.SessionsView{
		PageRecords:    m.sessionsPageRecords(),
		Cursor:         m.sessionCursor,
		Page:           m.sessionsPage + 1,
		PageCount:      m.sessionsPageCount(),
		TotalCount:     len(m.sessions),
		ErrMsg:          m.sessionsErr,
		Transcribing:    m.transcribing,
		StatusNote:      m.transcribeNote,
		DesktopNote:     m.sessionsDesktopNote,
		Width:           m.width,
		Height:          m.height,
		OpenerPicker:    m.sessionsOpenerPicker,
		OpenerCursor:    m.sessionsOpenerCursor,
		OpenerChoices:   m.sessionsOpenerChoices(),
		CurrentOpener:   open.CurrentOpenerID(m.deps.Config.Desktop),
		OpenerDetected:  open.Detected(m.deps.Config.Desktop, open.KindFolder),
		DeleteConfirm:   m.sessionsDeleteConfirm,
		DeleteCursor:    m.sessionsDeleteCursor,
	}
	if m.sessionsDeleteConfirm {
		v.DeleteID = rec.ID
		v.DeletePath = rec.Dir
	}
	return v
}

func (m Model) sessionsFooter() components.SessionsFooterMode {
	if m.sessionsDeleteConfirm {
		return components.SessionsFooterDeleteConfirm
	}
	if m.sessionsOpenerPicker {
		return components.SessionsFooterOpenerPicker
	}
	return components.SessionsFooterNormal
}

func (m Model) configView() components.ConfigMenuView {
	cfg := m.deps.Config
	sections := make([]components.ConfigSectionPanel, 0, configSectionCount)
	for s := 0; s < configSectionCount; s++ {
		fields := cfgFields(s)
		rows := make([]components.ConfigFieldRow, 0, len(fields))
		for i, f := range fields {
			row := components.ConfigFieldRow{
				Label:    f.label,
				Value:    cfgFieldValue(f, cfg),
				Selected: s == m.configSection && i == m.configCursor,
				Kind:     configFieldKindName(f.kind),
			}
			if f.kind == fieldList && f.list != nil {
				items := f.list(cfg)
				for j, item := range items {
					row.ListItems = append(row.ListItems, components.ConfigListItem{
						Text:     item,
						Selected: s == m.configSection && i == m.configCursor && j == m.configListCursor,
					})
				}
			}
			rows = append(rows, row)
		}
		sections = append(sections, components.ConfigSectionPanel{
			Label:   configSectionLabels[s],
			Focused: s == m.configSection,
			Fields:  rows,
		})
	}

	modalTitle := ""
	if m.configModalOpen {
		if f, ok := m.currentCfgField(); ok {
			modalTitle = f.label
		}
	}

	return components.ConfigMenuView{
		Path:          m.deps.ConfigPath,
		Sections:      sections,
		ModalOpen:     m.configModalOpen,
		ModalTitle:    modalTitle,
		ModalOptions:  m.configModalOptions,
		ModalCursor:   m.configModalCursor,
		Editing:       m.configEditing,
		InputValue:    m.configInput,
		Width:         m.width,
		Height:        m.height,
	}
}

func (m Model) configFooter() components.ConfigFooterMode {
	if m.configModalOpen {
		return components.ConfigFooterModal
	}
	if m.configEditing {
		return components.ConfigFooterEditing
	}
	return components.ConfigFooterNormal
}

func configFieldKindName(k cfgFieldKind) string {
	switch k {
	case fieldBool:
		return "bool"
	case fieldEnum:
		return "enum"
	case fieldText:
		return "text"
	case fieldInt:
		return "int"
	case fieldList:
		return "list"
	default:
		return "readonly"
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
