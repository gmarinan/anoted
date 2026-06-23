package setup

import "anoted/internal/platform"

// DetectionChoice is one meeting-detection option in setup.
type DetectionChoice struct {
	Mode        string
	Label       string
	Description string
	Recommended bool
}

// DetectionChoices returns platform-appropriate detection options.
func DetectionChoices(plat platform.Info) []DetectionChoice {
	if plat.OS == platform.OSWindows {
		return []DetectionChoice{
			{
				Mode:        DetMic,
				Label:       "Mic activity",
				Description: "Detect Meet/Teams when they use your microphone (uses audio sessions + titles)",
				Recommended: true,
			},
			{
				Mode:        DetWindow,
				Label:       "Window titles only",
				Description: "Browser tab titles via PowerShell (no mic signal)",
				Recommended: false,
			},
			{
				Mode:        DetNone,
				Label:       "Skip",
				Description: "Manual recording only (press r in the TUI)",
				Recommended: false,
			},
		}
	}
	choices := []DetectionChoice{
		{
			Mode:        DetMic,
			Label:       "PipeWire mic",
			Description: "When Meet/Teams uses your microphone (recommended)",
			Recommended: true,
		},
		{
			Mode:        DetWindow,
			Label:       "Window titles",
			Description: "Browser tab titles via xdotool/wmctrl (X11)",
			Recommended: false,
		},
		{
			Mode:        DetBoth,
			Label:       "Both",
			Description: "Mic first, then window titles",
			Recommended: false,
		},
		{
			Mode:        DetNone,
			Label:       "Skip",
			Description: "Manual recording only",
			Recommended: false,
		},
	}
	return choices
}

// DefaultDetectionMode returns the recommended mode for the platform.
func DefaultDetectionMode(plat platform.Info) string {
	for _, c := range DetectionChoices(plat) {
		if c.Recommended {
			return c.Mode
		}
	}
	return DetMic
}
