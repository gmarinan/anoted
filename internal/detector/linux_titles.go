//go:build linux

package detector

import (
	"context"
	"os/exec"
	"strings"
	"sync"
)

// The tool paths cannot change while anoted runs, and LookPath walks $PATH on
// every call — this runs on every detection poll, so resolve once (same
// pattern as pactlPath in linux_mic.go).
var (
	xdotoolPath = sync.OnceValues(func() (string, error) { return exec.LookPath("xdotool") })
	wmctrlPath  = sync.OnceValues(func() (string, error) { return exec.LookPath("wmctrl") })
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
		// path needs two. Both see the same X11 information.
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
	path, err := xdotoolPath()
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

	// The active window is the most likely match, so resolve it first; chaining
	// getwindowname after getactivewindow does it in one exec.
	if out, err := exec.CommandContext(ctx, path, "getactivewindow", "getwindowname").Output(); err == nil {
		add(string(out))
	}

	// Chaining getwindowname %@ resolves every matched window's name inside
	// this single exec. This used to fork one xdotool per window on the
	// desktop, on every poll.
	out, err := exec.CommandContext(ctx, path, "search", "--name", ".*", "getwindowname", "%@").Output()
	if err != nil {
		return titles
	}
	for _, line := range strings.Split(string(out), "\n") {
		add(line)
	}
	return titles
}

func titlesFromWmctrl(ctx context.Context) []string {
	path, err := wmctrlPath()
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
