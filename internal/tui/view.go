package tui

import (
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"anoted/internal/config"
	"anoted/internal/open"
	"anoted/internal/transcribe"
	"anoted/internal/tui/components"
)

func (m Model) View() tea.View {
	var content strings.Builder

	tab := components.ScreenToTab(string(m.screen))
	content.WriteString(components.Header(m.deps.Platform.Subtitle()))
	content.WriteString("\n")
	content.WriteString(components.TabBar(tab))
	content.WriteString("\n\n")

	switch m.screen {
	case ScreenDoctor:
		content.WriteString(m.doctorView().View())
	case ScreenConfig:
		content.WriteString(m.configView().View())
	default:
		content.WriteString(m.homeView().View())
	}

	content.WriteString("\n")
	footer := m.appFooter(tab)
	content.WriteString(components.FooterBar(footer, m.width))

	body := components.PadView(content.String(), m.width, m.height)
	body = m.setupWizardOverlay(body)
	body = m.quitConfirmOverlay(body)
	v := tea.NewView(body)
	v.AltScreen = true
	v.MouseMode = tea.MouseModeCellMotion
	v.WindowTitle = "anoted"
	if m.deps.Config.Privacy.ShowRecordingIndicator && m.recording {
		v.WindowTitle = "anoted ● RECORDING"
	}
	return v
}

func (m Model) appFooter(tab components.TabID) string {
	if m.quitConfirmOpen {
		return components.QuitConfirmFooter()
	}
	return components.FooterForTab(tab, m.awaitingRecordConfirm, m.sessionsFooter(), m.doctorFooter(), m.configFooter(), m.configSavedMsg, m.configErr, m.width)
}

func (m Model) homeView() components.HomeView {
	duration := time.Duration(0)
	if m.recording && !m.recordStart.IsZero() {
		duration = time.Since(m.recordStart)
	}
	return components.HomeView{
		AppState:        string(m.appState),
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
		Height:          m.height,

		SystemBands:    m.systemBands,
		MicBands:       m.micBands,
		LevelEnabled:   config.LevelMeterEnabled(m.deps.Config),
		LevelAvailable: m.deps.LevelMonitor != nil && m.deps.LevelMonitor.Available(),
		MonitorWarn:    m.audioMonitorWarn,
		LevelFrame:     m.levelFrame,

		Sessions: m.sessionsPanel(),
	}
}

func (m Model) sessionsPanel() components.SessionsView {
	rec, _ := m.selectedSession()
	preview := ""
	if rec.Dir != "" && transcribe.HasTranscript(rec.Dir, m.deps.Config.Transcription) && !(m.transcribeActive && rec.Dir == m.transcribeSessionDir) {
		if text, err := transcribe.ReadPreview(rec.Dir, m.deps.Config.Transcription, 12); err == nil {
			preview = text
		}
	}
	v := components.SessionsView{
		PageRecords:          m.sessionsPageRecords(),
		Cursor:               m.sessionCursor,
		Page:                 m.sessionsPage + 1,
		PageCount:            m.sessionsPageCount(),
		TotalCount:           len(m.sessions),
		ErrMsg:               m.sessionsErr,
		DesktopNote:          m.sessionsDesktopNote,
		Width:                m.width,
		Height:               m.height,
		OpenerPicker:         m.sessionsOpenerPicker,
		OpenerCursor:         m.sessionsOpenerCursor,
		OpenerChoices:        m.sessionsOpenerChoices(),
		CurrentOpener:        open.CurrentOpenerID(m.deps.Config.Desktop),
		OpenerDetected:       open.Detected(m.deps.Config.Desktop, open.KindFolder),
		DeleteConfirm:        m.sessionsDeleteConfirm,
		DeleteCursor:         m.sessionsDeleteCursor,
		TranscribeActive:     m.transcribeActive,
		TranscribeSessionDir: m.transcribeSessionDir,
		TranscribePercent:    m.transcribePercent,
		TranscribeETA:        m.transcribeETA,
		TranscribeBlink:      m.transcribeBlink,
		TranscribeLog:        append([]string(nil), m.transcribeLog...),
		TranscribeErr:        m.transcribeErr,
		TranscribeErrDir:     m.transcribeSessionDir,
		Transcription:        m.deps.Config.Transcription,
		PreviewText:          preview,
	}
	if m.sessionsDeleteConfirm {
		v.DeleteID = rec.ID
		v.DeletePath = rec.Dir
	}
	return v
}

func (m Model) doctorView() components.DoctorView {
	backend := m.deps.Recorder.Name()
	if m.recStatus.Backend != "" {
		backend = m.recStatus.Backend
	}
	return components.DoctorView{
		Report:               m.doctorReport,
		AppState:             string(m.appState),
		Platform:             m.deps.Platform.Name(),
		Backend:              backend,
		Provider:             displayProvider(m.provider, m.detection.Title),
		SystemDevice:         m.systemDevice,
		MicDevice:            m.micDevice,
		DetectionWarn:        m.detection.Warning,
		Width:                m.width,
		Height:               m.height,
		WhisperInstallActive: m.whisperInstallActive,
		WhisperInstallLog:    append([]string(nil), m.whisperInstallLog...),
		WhisperInstallErr:    m.whisperInstallErr,
		WhisperCanInstall:    m.doctorWhisperOffer(),
		GPUInstallActive:     m.gpuInstallActive,
		GPUInstallLog:        m.visibleGPUInstallLog(8),
		GPUInstallErr:        m.gpuInstallErr,
		GPUCanInstall:        m.doctorGPUOffer(),
	}
}

func (m Model) sessionsFooter() components.SessionsFooterMode {
	if m.sessionsDeleteConfirm {
		return components.SessionsFooterDeleteConfirm
	}
	if m.sessionsOpenerPicker {
		return components.SessionsFooterOpenerPicker
	}
	if m.transcribeActive {
		return components.SessionsFooterTranscribing
	}
	return components.SessionsFooterNormal
}

func (m Model) doctorFooter() components.DoctorFooterMode {
	if m.gpuInstallActive {
		return components.DoctorFooterInstallingGPU
	}
	if m.whisperInstallActive {
		return components.DoctorFooterInstalling
	}
	whisper := m.doctorWhisperOffer()
	gpu := m.doctorGPUOffer()
	if whisper && gpu {
		return components.DoctorFooterCanInstallBoth
	}
	if gpu {
		return components.DoctorFooterCanInstallGPU
	}
	if whisper {
		return components.DoctorFooterCanInstall
	}
	return components.DoctorFooterNormal
}

func (m Model) configView() components.ConfigMenuView {
	cfg := m.deps.Config
	sections := make([]components.ConfigSectionPanel, 0, configSectionCount)
	for s := 0; s < configSectionCount; s++ {
		fields := cfgFields(s)
		rows := make([]components.ConfigFieldRow, 0, len(fields))
		for i, f := range fields {
			value := cfgFieldValue(f, cfg)
			if f.kind == fieldDevice {
				if f.deviceSection == components.AudioSectionOutput && m.systemDevice != "" {
					value = m.systemDevice
				}
				if f.deviceSection == components.AudioSectionMic && m.micDevice != "" {
					value = m.micDevice
				}
			}
			row := components.ConfigFieldRow{
				Label:    f.label,
				Value:    value,
				Selected: s == m.configSection && i == m.configCursor,
				Kind:     configFieldKindName(f.kind),
			}
			if f.editable != nil && !f.editable(cfg) {
				row.Kind = "readonly"
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

	deviceTitle := ""
	if m.configDevicePickerOpen {
		if m.configDeviceSection == components.AudioSectionOutput {
			deviceTitle = "system_monitor"
		} else {
			deviceTitle = "microphone"
		}
	}

	return components.ConfigMenuView{
		Path:             m.deps.ConfigPath,
		Sections:         sections,
		ModalOpen:        m.configModalOpen,
		ModalTitle:       modalTitle,
		ModalOptions:     m.configModalOptions,
		ModalCursor:      m.configModalCursor,
		DevicePickerOpen: m.configDevicePickerOpen,
		DeviceTitle:      deviceTitle,
		DeviceSection:    m.configDeviceSection,
		DeviceCatalog:    m.configDeviceCatalog,
		DeviceCursor:     m.configDeviceCursor,
		DeviceLoading:    m.configDeviceLoading,
		DeviceErr:        m.configDeviceErr,
		SystemMonitor:    m.deps.Config.Audio.SystemMonitor,
		Microphone:       m.deps.Config.Audio.Microphone,
		Editing:          m.configEditing,
		InputValue:       m.configInput,
		Width:            m.width,
		Height:           m.height,
	}
}

func (m Model) configFooter() components.ConfigFooterMode {
	if m.configDevicePickerOpen {
		return components.ConfigFooterDevicePicker
	}
	if m.configModalOpen {
		return components.ConfigFooterModal
	}
	if m.configEditing {
		return components.ConfigFooterEditing
	}
	if f, ok := m.currentCfgField(); ok && f.kind == fieldPath {
		return components.ConfigFooterPathField
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
	case fieldDevice:
		return "device"
	case fieldPath:
		return "path"
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
