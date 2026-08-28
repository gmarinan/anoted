package components

import (
	"fmt"
	"strings"

	"anoted/internal/audio"
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
	// MaxRows bounds how many devices are drawn at once. Zero means unbounded,
	// which is only appropriate for short, fixed lists.
	MaxRows int
}

// deviceWindow returns the slice of indices to draw, keeping Cursor visible.
//
// A PipeWire desktop routinely exposes 20-40 nodes and renderDevice adds a line
// per linked application on top, so an unwindowed list ran off the bottom of the
// terminal: the cursor could sit on a device that was not being drawn and the
// user was choosing blind.
func (p AudioPanel) deviceWindow(n int) (int, int) {
	if p.MaxRows <= 0 || n <= p.MaxRows {
		return 0, n
	}
	start := p.Cursor - p.MaxRows/2
	if start < 0 {
		start = 0
	}
	if start+p.MaxRows > n {
		start = n - p.MaxRows
	}
	return start, start + p.MaxRows
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
	start, end := p.deviceWindow(len(devices))
	var lines []string
	for i := start; i < end; i++ {
		lines = append(lines, p.renderDevice(devices, i, devices[i], selected))
	}
	if start > 0 || end < len(devices) {
		lines = append(lines, subtleStyle.Render(
			fmt.Sprintf("  %d/%d  ↑↓ to scroll", p.Cursor+1, len(devices))))
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
