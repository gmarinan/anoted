package platform

import "time"

// OS identifies the host operating system family.
type OS string

const (
	OSLinux   OS = "linux"
	OSWindows OS = "windows"
	OSUnknown OS = "unknown"
)

// Info describes the runtime platform.
type Info struct {
	OS          OS
	IsWSL2      bool
	DisplayName string
	Session     string // x11, wayland, windows
}

// Detect returns platform information for the current host.
func Detect() Info {
	return detect()
}

// Name returns a human-readable platform name.
func (i Info) Name() string {
	if i.DisplayName != "" {
		return i.DisplayName
	}
	return string(i.OS)
}

// Subtitle returns the platform and audio backend label for the TUI header.
func (i Info) Subtitle() string {
	switch i.OS {
	case OSWindows:
		return "Windows · WASAPI"
	case OSLinux:
		if i.IsWSL2 {
			return "WSL2 · WASAPI"
		}
		return "Linux · PipeWire"
	default:
		return "Unknown"
	}
}

// WindowSizePollInterval returns how often the TUI should poll terminal size
// with term.GetSize. Windows lacks reliable SIGWINCH; Linux gets resize events
// from Bubble Tea and does not need polling.
func (i Info) WindowSizePollInterval() time.Duration {
	if i.OS == OSWindows {
		return 200 * time.Millisecond
	}
	return 0
}

// ClearScreenOnResize reports whether a terminal resize should trigger an
// explicit screen clear before redraw. PadView already pads to the terminal
// height; an extra clear on Linux tends to flicker, especially on Home where
// the level meter redraws frequently.
func (i Info) ClearScreenOnResize() bool {
	return i.OS == OSWindows
}
