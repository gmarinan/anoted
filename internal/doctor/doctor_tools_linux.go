//go:build linux

package doctor

import "anoted/internal/config"

func optionalToolChecks(_ config.Config) []Check {
	tools := []string{"ffmpeg", "pw-cat", "pactl", "xdotool", "wmctrl"}
	checks := make([]Check, 0, len(tools))
	for _, tool := range tools {
		checks = append(checks, commandCheck(tool))
	}
	return checks
}
