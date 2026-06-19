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

// AudioView renders the Audio tab.
type AudioView struct {
	Catalog       audio.Catalog
	Section       AudioSection
	Cursor        int
	SystemMonitor string
	Microphone    string
	Loading       bool
	ErrMsg        string
	SavedMsg      string
	MonitorWarn   string
	Width         int
}

func (v AudioView) View() string {
	var b strings.Builder
	b.WriteString(Header())
	b.WriteString("\n")
	b.WriteString(TabBar(TabAudio))
	b.WriteString("\n\n")

	if v.Loading {
		b.WriteString(Box("Output devices", subtleStyle.Render("Loading…"), v.Width))
		return b.String()
	}
	if v.ErrMsg != "" {
		b.WriteString(errStyle.Render("✗ " + v.ErrMsg))
		b.WriteString("\n\n")
	}
	if v.MonitorWarn != "" {
		b.WriteString(warnStyle.Render("⚠ " + v.MonitorWarn))
		b.WriteString("\n\n")
	}

	b.WriteString(v.deviceBox("Output devices (system audio)", v.Catalog.Outputs, v.Section == AudioSectionOutput, v.SystemMonitor))
	b.WriteString("\n\n")
	b.WriteString(v.deviceBox("Input devices (microphone)", v.Catalog.Microphones, v.Section == AudioSectionMic, v.Microphone))

	if v.SavedMsg != "" {
		b.WriteString("\n\n")
		b.WriteString(okStyle.Render("✓ " + v.SavedMsg))
	}

	b.WriteString("\n\n")
	b.WriteString(subtleStyle.Render("Legend: "))
	b.WriteString(Badge("RUNNING", "ok"))
	b.WriteString(" ")
	b.WriteString(Badge("DEFAULT", "warn"))
	b.WriteString("  ·  ")
	b.WriteString(FooterHint("↑↓", "navigate"))
	b.WriteString("  ")
	b.WriteString(FooterHint("Tab", "switch section"))
	b.WriteString("  ")
	b.WriteString(FooterHint("Enter", "select"))

	return b.String()
}

func (v AudioView) deviceBox(title string, devices []audio.Device, focused bool, selectedID string) string {
	prefix := "  "
	if focused {
		prefix = "▸ "
	}
	boxTitle := prefix + strings.ToUpper(title)
	if focused {
		boxTitle = boxTitleStyle.Render(boxTitle)
	} else {
		boxTitle = labelStyle.Render(boxTitle)
	}

	var lines []string
	if len(devices) == 0 {
		lines = append(lines, subtleStyle.Render("  (no devices found)"))
	} else {
		for i, d := range devices {
			lines = append(lines, v.renderDevice(devices, i, d, focused, selectedID))
		}
	}
	body := boxTitle + "\n" + strings.Join(lines, "\n")
	return boxStyle.Width(v.Width).Render(body)
}

func (v AudioView) renderDevice(all []audio.Device, idx int, d audio.Device, focused bool, selectedID string) string {
	marker := "  "
	if focused && idx == v.Cursor {
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

	if focused && idx == v.Cursor {
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
