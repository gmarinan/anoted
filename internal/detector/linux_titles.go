//go:build linux

package detector

import (
	"context"
	"os/exec"
	"strings"
)

func (d *linuxDetector) windowTitles(ctx context.Context) []string {
	switch d.cfg.WindowTool {
	case "none":
		return nil
	case "xdotool":
		return titlesFromXdotool(ctx)
	case "wmctrl":
		return titlesFromWmctrl(ctx)
	default: // auto
		// wmctrl first: one exec returns every window title, while the xdotool
		// path needs one exec per window. Both see the same X11 information.
		if t := titlesFromWmctrl(ctx); len(t) > 0 {
			return t
		}
		return titlesFromXdotool(ctx)
	}
}

// titlesFromXdotool lists every window title, not just the focused one.
//
// It used to return early with only the active window. That made detection
// depend on what the user was looking at: alt-tabbing from a Meet tab to a
// terminal dropped the meeting title, the next poll reported "not in a
// meeting", and the end-of-meeting grace timer stopped the recording partway
// through the call.
func titlesFromXdotool(ctx context.Context) []string {
	path, err := exec.LookPath("xdotool")
	if err != nil {
		return nil
	}

	var titles []string
	seen := make(map[string]bool)
	add := func(title string) {
		title = strings.TrimSpace(title)
		if title == "" || seen[title] {
			return
		}
		seen[title] = true
		titles = append(titles, title)
	}

	nameOf := func(id string) string {
		out, err := exec.CommandContext(ctx, path, "getwindowname", id).Output()
		if err != nil {
			return ""
		}
		return string(out)
	}

	// The active window is the most likely match, so resolve it first and let
	// the sweep below fill in the rest.
	if winOut, err := exec.CommandContext(ctx, path, "getactivewindow").Output(); err == nil {
		if winID := strings.TrimSpace(string(winOut)); winID != "" {
			add(nameOf(winID))
		}
	}

	out, err := exec.CommandContext(ctx, path, "search", "--name", ".*").Output()
	if err != nil {
		return titles
	}
	for _, id := range strings.Fields(string(out)) {
		add(nameOf(id))
	}
	return titles
}

func titlesFromWmctrl(ctx context.Context) []string {
	path, err := exec.LookPath("wmctrl")
	if err != nil {
		return nil
	}
	out, err := exec.CommandContext(ctx, path, "-l").Output()
	if err != nil {
		return nil
	}
	var titles []string
	for _, line := range strings.Split(string(out), "\n") {
		parts := strings.Fields(line)
		if len(parts) >= 4 {
			titles = append(titles, strings.Join(parts[3:], " "))
		}
	}
	return titles
}
