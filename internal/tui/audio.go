package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
	"meetctl/internal/audio"
	"meetctl/internal/config"
	"meetctl/internal/tui/components"
)

func (m Model) openAudioScreen() (Model, tea.Cmd) {
	m.screen = ScreenAudio
	m.audioSection = components.AudioSectionOutput
	m.audioCursor = 0
	m.audioLoading = true
	m.audioErr = ""
	m.audioSaved = ""
	m.audioMonitorWarn = m.deps.Audio.MonitorWarning(m.deps.Config.Audio.SystemMonitor)
	return m, tea.Batch(loadAudioCatalogCmd(m), resolveDeviceLabelsCmd(m))
}

func (m Model) closeAudioScreen() Model {
	m.screen = ScreenMain
	m.audioSaved = ""
	return m
}

func (m Model) handleAudioKey(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "o", "esc":
		return m.closeAudioScreen(), resolveDeviceLabelsCmd(m)
	case "tab":
		if m.audioSection == components.AudioSectionOutput {
			m.audioSection = components.AudioSectionMic
		} else {
			m.audioSection = components.AudioSectionOutput
		}
		m.audioCursor = m.cursorForSelection()
		return m, nil
	case "up", "k":
		if m.audioCursor > 0 {
			m.audioCursor--
		}
		return m, nil
	case "down", "j":
		max := m.audioListLen() - 1
		if m.audioCursor < max {
			m.audioCursor++
		}
		return m, nil
	case "enter", " ":
		return m, m.selectAudioDevice()
	case "R":
		m.audioLoading = true
		m.audioErr = ""
		return m, loadAudioCatalogCmd(m)
	}
	return m, nil
}

func (m Model) audioListLen() int {
	if m.audioSection == components.AudioSectionOutput {
		return len(m.audioCatalog.Outputs)
	}
	return len(m.audioCatalog.Microphones)
}

func (m Model) cursorForSelection() int {
	var devices []audio.Device
	var selected string
	if m.audioSection == components.AudioSectionOutput {
		devices = m.audioCatalog.Outputs
		selected = m.deps.Config.Audio.SystemMonitor
	} else {
		devices = m.audioCatalog.Microphones
		selected = m.deps.Config.Audio.Microphone
	}
	for i, d := range devices {
		if d.ID == selected {
			return i
		}
	}
	return 0
}

func (m Model) selectAudioDevice() tea.Cmd {
	section := m.audioSection
	cursor := m.audioCursor
	cfg := m.deps.Config
	path := m.deps.ConfigPath
	catalog := m.audioCatalog

	return func() tea.Msg {
		var devices []audio.Device
		if section == components.AudioSectionOutput {
			devices = catalog.Outputs
		} else {
			devices = catalog.Microphones
		}
		if cursor < 0 || cursor >= len(devices) {
			return configSavedMsg{err: fmt.Errorf("invalid selection")}
		}
		id := devices[cursor].ID
		if section == components.AudioSectionOutput {
			cfg.Audio.SystemMonitor = id
		} else {
			cfg.Audio.Microphone = id
		}
		if err := config.Save(path, cfg); err != nil {
			return configSavedMsg{err: err}
		}
		return configSavedMsg{cfg: cfg}
	}
}

func loadAudioCatalogCmd(m Model) tea.Cmd {
	p := m.deps.Audio
	return func() tea.Msg {
		cat, err := p.List()
		return audioCatalogMsg{catalog: cat, err: err}
	}
}

func resolveDeviceLabelsCmd(m Model) tea.Cmd {
	p := m.deps.Audio
	cfg := m.deps.Config
	return func() tea.Msg {
		sys, mic, err := p.Resolve(cfg.Audio.SystemMonitor, cfg.Audio.Microphone)
		if err != nil {
			return deviceLabelsMsg{err: err}
		}
		return deviceLabelsMsg{
			system: formatDeviceLabel(cfg.Audio.SystemMonitor, sys),
			mic:    formatDeviceLabel(cfg.Audio.Microphone, mic),
		}
	}
}

func formatDeviceLabel(configured, resolved string) string {
	if configured == "" {
		return audio.AutoLabel + " → " + resolved
	}
	return configured
}

func (m Model) handleAudioCatalog(msg audioCatalogMsg) (Model, tea.Cmd) {
	m.audioLoading = false
	if msg.err != nil {
		m.audioErr = msg.err.Error()
		return m, nil
	}
	m.audioCatalog = msg.catalog
	m.audioCursor = m.cursorForSelection()
	return m, nil
}

func (m Model) handleConfigSaved(msg configSavedMsg) (Model, tea.Cmd) {
	if msg.err != nil {
		m.audioErr = msg.err.Error()
		return m, nil
	}
	m.deps.Config = msg.cfg
	m.audioSaved = "saved to config"
	m.audioMonitorWarn = m.deps.Audio.MonitorWarning(msg.cfg.Audio.SystemMonitor)
	m.audioCursor = m.cursorForSelection()
	return m, resolveDeviceLabelsCmd(m)
}

func (m Model) handleDeviceLabels(msg deviceLabelsMsg) Model {
	if msg.err != nil {
		return m
	}
	m.systemDevice = msg.system
	m.micDevice = msg.mic
	return m
}
