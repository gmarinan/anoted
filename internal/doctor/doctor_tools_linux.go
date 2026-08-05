//go:build linux

package doctor

import (
	"os/exec"

	"anoted/internal/config"
)

func optionalToolChecks(_ config.Config) []Check {
	tools := []string{"pw-cat", "pactl", "xdotool", "wmctrl"}
	// ffmpeg is required rather than optional: every Linux capture backend mixes
	// the system and microphone inputs through it, so without ffmpeg there is no
	// working recorder — reporting it as merely optional hid that.
	checks := make([]Check, 0, len(tools)+1)
	if path, err := exec.LookPath("ffmpeg"); err != nil {
		checks = append(checks, Check{
			Name:   "ffmpeg",
			Status: "fail",
			Detail: "not found in PATH — required for audio capture on Linux",
		})
	} else {
		checks = append(checks, Check{Name: "ffmpeg", Status: "ok", Detail: path})
	}
	for _, tool := range tools {
		checks = append(checks, commandCheck(tool))
	}
	return checks
}
