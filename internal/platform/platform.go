package platform

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
