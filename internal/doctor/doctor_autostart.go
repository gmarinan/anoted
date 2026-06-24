package doctor

import (
	"os"
	"os/exec"
	"strings"

	"anoted/internal/autostart"
)

func autostartCheck() Check {
	if !autostart.Available() {
		return Check{Name: "launch_at_login", Status: "warn", Detail: "not supported on this platform"}
	}
	if autostart.Enabled() {
		path, err := autostart.Path()
		if err != nil {
			return Check{Name: "launch_at_login", Status: "ok", Detail: "enabled"}
		}
		detail := "enabled (" + path + ")"
		if i3SessionLikely() {
			return Check{
				Name:   "launch_at_login",
				Status: "warn",
				Detail: detail + " — i3 does not run ~/.config/autostart; add exec lines to ~/.config/i3/config",
			}
		}
		return Check{Name: "launch_at_login", Status: "ok", Detail: detail}
	}
	return Check{Name: "launch_at_login", Status: "warn", Detail: "disabled — run: anoted autostart enable --record"}
}

func i3SessionLikely() bool {
	if strings.Contains(strings.ToLower(os.Getenv("XDG_CURRENT_DESKTOP")), "i3") {
		return true
	}
	if strings.Contains(strings.ToLower(os.Getenv("DESKTOP_SESSION")), "i3") {
		return true
	}
	if _, err := exec.LookPath("i3-msg"); err != nil {
		return false
	}
	return exec.Command("pgrep", "-x", "i3").Run() == nil
}
