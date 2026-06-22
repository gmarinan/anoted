package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
	"meetctl/internal/audio"
	"meetctl/internal/config"
	"meetctl/internal/tui/components"
)

func (m Model) configDeviceCursorForSelection() int {
	var devices []audio.Device
	var selected string
	if m.configDeviceSection == components.AudioSectionOutput {
		devices = m.configDeviceCatalog.Outputs
		selected = m.deps.Config.Audio.SystemMonitor
	} else {
		devices = m.configDeviceCatalog.Microphones
		selected = m.deps.Config.Audio.Microphone
	}
	for i, d := range devices {
		if d.ID == selected {
			return i
		}
	}
	return 0
}

func (m Model) configDeviceListLen() int {
	if m.configDeviceSection == components.AudioSectionOutput {
		return len(m.configDeviceCatalog.Outputs)
	}
	return len(m.configDeviceCatalog.Microphones)
}

func (m Model) openConfigDevicePicker(section components.AudioSection) (tea.Model, tea.Cmd) {
	m.configDevicePickerOpen = true
	m.configDeviceSection = section
	m.configDeviceLoading = true
	m.configDeviceErr = ""
	m.configDeviceCursor = 0
	return m, loadAudioCatalogCmd(m)
}

func (m Model) handleConfigDeviceModalKey(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "esc":
		m.configDevicePickerOpen = false
		m.configDeviceLoading = false
		m.configDeviceErr = ""
		return m, nil
	case "up", "k":
		if m.configDeviceCursor > 0 {
			m.configDeviceCursor--
		}
		return m, nil
	case "down", "j":
		max := m.configDeviceListLen() - 1
		if m.configDeviceCursor < max {
			m.configDeviceCursor++
		}
		return m, nil
	case "enter", " ":
		m.configDevicePickerOpen = false
		return m, m.selectConfigAudioDevice()
	}
	return m, nil
}

func (m Model) selectConfigAudioDevice() tea.Cmd {
	section := m.configDeviceSection
	cursor := m.configDeviceCursor
	cfg := m.deps.Config
	path := m.deps.ConfigPath
	catalog := m.configDeviceCatalog

	return func() tea.Msg {
		var devices []audio.Device
		if section == components.AudioSectionOutput {
			devices = catalog.Outputs
		} else {
			devices = catalog.Microphones
		}
		if cursor < 0 || cursor >= len(devices) {
			return configMenuSaveMsg{err: fmt.Errorf("invalid device selection")}
		}
		id := devices[cursor].ID
		if section == components.AudioSectionOutput {
			cfg.Audio.SystemMonitor = id
		} else {
			cfg.Audio.Microphone = id
		}
		if err := config.Save(path, cfg); err != nil {
			return configMenuSaveMsg{err: err}
		}
		return configMenuSaveMsg{cfg: cfg}
	}
}
