//go:build linux

package platform

import (
	"os"
	"strings"
)

func detect() Info {
	info := Info{
		OS:          OSLinux,
		DisplayName: "Linux",
		Session:     detectLinuxSession(),
	}
	if isWSL2() {
		info.IsWSL2 = true
		info.DisplayName = "Linux (WSL2)"
	}
	return info
}

func isWSL2() bool {
	data, err := os.ReadFile("/proc/version")
	if err != nil {
		return false
	}
	lower := strings.ToLower(string(data))
	return strings.Contains(lower, "microsoft") || strings.Contains(lower, "wsl")
}

func detectLinuxSession() string {
	switch strings.ToLower(os.Getenv("XDG_SESSION_TYPE")) {
	case "wayland":
		return "wayland"
	case "x11":
		return "x11"
	default:
		if os.Getenv("WAYLAND_DISPLAY") != "" {
			return "wayland"
		}
		if os.Getenv("DISPLAY") != "" {
			return "x11"
		}
		return "unknown"
	}
}
