package components

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"meetctl/internal/audio"
)

// AudioSection is which device list has focus.
type AudioSection int

const (
	AudioSectionOutput AudioSection = iota
	AudioSectionMic
)

// AudioPanel renders the device picker screen.
type AudioPanel struct {
	Catalog       audio.Catalog
	Section       AudioSection
	Cursor        int
	SystemMonitor string // configured monitor ID (empty = auto)
	Microphone    string
	Loading       bool
	ErrMsg        string
	SavedMsg      string
	MonitorWarn   string
	Width         int
}

func (p AudioPanel) View() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Audio devices"))
	b.WriteString("\n")
	b.WriteString(labelStyle.Render("PipeWire nodes · Enter saves · pick the RUNNING output where apps play"))
	b.WriteString("\n\n")

	if p.Loading {
		b.WriteString(labelStyle.Render("Loading devices..."))
		return b.String()
	}
	if p.ErrMsg != "" {
		b.WriteString(errStyle.Render("✗ " + p.ErrMsg))
		b.WriteString("\n\n")
	}
	if p.MonitorWarn != "" {
		b.WriteString(warnStyle.Render("⚠ " + p.MonitorWarn))
		b.WriteString("\n\n")
	}

	b.WriteString(p.sectionHeader("Output devices (system audio)", p.Section == AudioSectionOutput))
	b.WriteString("\n")
	b.WriteString(p.renderList(p.Catalog.Outputs, p.Section == AudioSectionOutput, p.SystemMonitor))
	b.WriteString("\n\n")

	b.WriteString(p.sectionHeader("Input devices (microphone)", p.Section == AudioSectionMic))
	b.WriteString("\n")
	b.WriteString(p.renderList(p.Catalog.Microphones, p.Section == AudioSectionMic, p.Microphone))

	if p.SavedMsg != "" {
		b.WriteString("\n\n")
		b.WriteString(okStyle.Render("✓ " + p.SavedMsg))
	}

	return b.String()
}

func (p AudioPanel) sectionHeader(title string, focused bool) string {
	prefix := "  "
	if focused {
		prefix = "▸ "
	}
	style := labelStyle
	if focused {
		style = titleStyle
	}
	return style.Render(prefix + title)
}

func (p AudioPanel) renderList(devices []audio.Device, focused bool, selectedID string) string {
	if len(devices) == 0 {
		return labelStyle.Render("  (no devices found)")
	}
	var lines []string
	for i, d := range devices {
		lines = append(lines, p.renderDevice(devices, i, d, focused, selectedID))
	}
	return strings.Join(lines, "\n")
}

func (p AudioPanel) renderDevice(all []audio.Device, idx int, d audio.Device, focused bool, selectedID string) string {
	marker := "  "
	if focused && idx == p.Cursor {
		marker = "> "
	}
	sel := "○"
	if deviceSelected(d.ID, selectedID) {
		sel = "●"
	}

	var parts []string
	if d.ID == audio.AutoID {
		parts = append(parts, fmt.Sprintf("%s%s %s", marker, sel, d.Name))
	} else {
		state := stateStyle(d.State).Render(fmt.Sprintf("%-9s", d.State))
		parts = append(parts, fmt.Sprintf("%s%s [%s] %s %s", marker, sel, d.NodeID, state, d.Name))
		if d.Format != "" {
			parts[len(parts)-1] += labelStyle.Render("  " + d.Format)
		}
		if d.IsDefault {
			parts[len(parts)-1] += okStyle.Render("  default")
		}
	}

	line := parts[0]
	if focused && idx == p.Cursor {
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

func stateStyle(state string) lipgloss.Style {
	switch strings.ToUpper(state) {
	case "RUNNING":
		return okStyle
	case "SUSPENDED", "IDLE":
		return warnStyle
	default:
		return labelStyle
	}
}

func deviceSelected(deviceID, configuredID string) bool {
	return deviceID == configuredID
}

// AudioHelpBar returns shortcuts for the audio screen.
func AudioHelpBar() string {
	keys := []string{
		"↑/↓ navigate", "Tab switch", "Enter select", "o back", "R refresh",
	}
	return labelStyle.Render(strings.Join(keys, "  ·  "))
}
