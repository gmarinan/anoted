package components

import (
	"fmt"
	"strings"

	"meetctl/internal/audio"
)

// AudioSection is which device list has focus.
type AudioSection int

const (
	AudioSectionOutput AudioSection = iota
	AudioSectionMic
)

// AudioPanel renders the device picker list (used inside Home audio box).
type AudioPanel struct {
	Catalog       audio.Catalog
	Section       AudioSection
	Cursor        int
	SystemMonitor string
	Microphone    string
}

func (p AudioPanel) renderDeviceList() string {
	var devices []audio.Device
	var selected string
	switch p.Section {
	case AudioSectionOutput:
		devices = p.Catalog.Outputs
		selected = p.SystemMonitor
	default:
		devices = p.Catalog.Microphones
		selected = p.Microphone
	}
	if len(devices) == 0 {
		return subtleStyle.Render("  (no devices found)")
	}
	var lines []string
	for i, d := range devices {
		lines = append(lines, p.renderDevice(devices, i, d, selected))
	}
	return strings.Join(lines, "\n")
}

func (p AudioPanel) renderDevice(all []audio.Device, idx int, d audio.Device, selectedID string) string {
	marker := "  "
	if idx == p.Cursor {
		marker = "> "
	}
	sel := "○"
	if deviceSelected(d.ID, selectedID) {
		sel = "●"
	}

	var line string
	if d.ID == audio.AutoID {
		line = fmt.Sprintf("%s%s %s", marker, sel, d.Name)
		if d.IsDefault {
			line += " " + Badge("DEFAULT", "warn")
		}
	} else {
		state := Badge(strings.ToUpper(d.State), badgeKindForState(d.State))
		line = fmt.Sprintf("%s%s [%s] %s %s", marker, sel, d.NodeID, state, d.Name)
		if d.Format != "" {
			line += labelStyle.Render("  " + d.Format)
		}
		if d.IsDefault {
			line += " " + Badge("DEFAULT", "warn")
		}
	}

	if idx == p.Cursor {
		line = valueStyle.Bold(true).Render(line)
	} else {
		line = valueStyle.Render(line)
	}

	var extra []string
	for _, app := range d.LinkedApps {
		extra = append(extra, labelStyle.Render("      └ "+app))
	}
	if len(extra) > 0 {
		return line + "\n" + strings.Join(extra, "\n")
	}
	return line
}

func badgeKindForState(state string) string {
	switch strings.ToUpper(state) {
	case "RUNNING":
		return "running"
	case "SUSPENDED", "IDLE":
		return "warn"
	default:
		return ""
	}
}

func deviceSelected(deviceID, configuredID string) bool {
	return deviceID == configuredID
}
