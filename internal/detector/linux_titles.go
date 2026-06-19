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
		if t := titlesFromXdotool(ctx); len(t) > 0 {
			return t
		}
		return titlesFromWmctrl(ctx)
	}
}

func titlesFromXdotool(ctx context.Context) []string {
	path, err := exec.LookPath("xdotool")
	if err != nil {
		return nil
	}
	winOut, err := exec.CommandContext(ctx, path, "getactivewindow").Output()
	if err == nil {
		winID := strings.TrimSpace(string(winOut))
		if winID != "" {
			nameOut, err := exec.CommandContext(ctx, path, "getwindowname", winID).Output()
			if err == nil && len(nameOut) > 0 {
				return []string{strings.TrimSpace(string(nameOut))}
			}
		}
	}
	out, err := exec.CommandContext(ctx, path, "search", "--name", ".*").Output()
	if err != nil {
		return nil
	}
	var titles []string
	for _, id := range strings.Fields(string(out)) {
		nameOut, err := exec.CommandContext(ctx, path, "getwindowname", id).Output()
		if err == nil {
			title := strings.TrimSpace(string(nameOut))
			if title != "" {
				titles = append(titles, title)
			}
		}
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
