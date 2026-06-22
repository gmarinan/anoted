package tui

import (
	tea "charm.land/bubbletea/v2"
	"meetctl/internal/audio"
)

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

func (m Model) handleAudioCatalog(msg audioCatalogMsg) (tea.Model, tea.Cmd) {
	if m.configDevicePickerOpen || m.configDeviceLoading {
		m.configDeviceLoading = false
		if msg.err != nil {
			m.configDeviceErr = msg.err.Error()
			return m, nil
		}
		m.configDeviceCatalog = msg.catalog
		m.configDeviceCursor = m.configDeviceCursorForSelection()
		return m, nil
	}
	return m, nil
}

func (m Model) handleDeviceLabels(msg deviceLabelsMsg) Model {
	if msg.err != nil {
		return m
	}
	m.systemDevice = msg.system
	m.micDevice = msg.mic
	return m
}

func (m Model) handleConfigSaved(msg configSavedMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		return m, nil
	}
	m.deps.Config = msg.cfg
	m.audioMonitorWarn = m.deps.Audio.MonitorWarning(msg.cfg.Audio.SystemMonitor)
	return m, resolveDeviceLabelsCmd(m)
}
