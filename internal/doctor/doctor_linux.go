//go:build linux

package doctor

import (
	"os/exec"

	"meetctl/internal/config"
	"meetctl/internal/platform"
	"meetctl/internal/recorder"
)

func audioDeviceChecks(cfg config.Config) []Check {
	monitor, mic, err := recorder.ListAudioDevices(cfg)
	if err != nil {
		return []Check{{Name: "audio_devices", Status: "fail", Detail: err.Error()}}
	}
	checks := []Check{
		{Name: "system_monitor", Status: "ok", Detail: monitor},
		{Name: "microphone", Status: "ok", Detail: mic},
	}
	if cfg.Audio.SystemMonitor != "" {
		checks = append(checks, Check{Name: "configured_system_monitor", Status: "ok", Detail: cfg.Audio.SystemMonitor})
	}
	if cfg.Audio.Microphone != "" {
		checks = append(checks, Check{Name: "configured_microphone", Status: "ok", Detail: cfg.Audio.Microphone})
	}
	return checks
}

func detectionChecks(plat platform.Info, cfg config.Config) []Check {
	mode := cfg.Detection.Mode
	if mode == "" {
		mode = "mic"
	}

	if mode == "none" {
		return []Check{{
			Name:   "meeting_detection",
			Status: "ok",
			Detail: "disabled (manual record only)",
		}}
	}

	var checks []Check
	checks = append(checks, Check{
		Name:   "detection_mode",
		Status: "ok",
		Detail: mode,
	})

	if mode == "mic" || mode == "both" {
		if _, err := exec.LookPath("pactl"); err != nil {
			checks = append(checks, Check{
				Name:   "meeting_detection_mic",
				Status: "warn",
				Detail: "pactl not found — install pipewire-pulse",
			})
		} else {
			p, _ := exec.LookPath("pactl")
			checks = append(checks, Check{
				Name:   "meeting_detection_mic",
				Status: "ok",
				Detail: "PipeWire mic activity via " + p,
			})
		}
	}

	if mode == "window" || mode == "both" {
		if plat.Session == "wayland" {
			checks = append(checks, Check{
				Name:   "meeting_detection_window",
				Status: "warn",
				Detail: "window titles limited on Wayland",
			})
		} else if plat.Session == "x11" {
			checks = append(checks, windowToolCheck(cfg.Detection.WindowTool)...)
		}
	}

	return checks
}

func windowToolCheck(want string) []Check {
	if want == "none" {
		return []Check{{
			Name:   "meeting_detection_window",
			Status: "warn",
			Detail: "window tool disabled in config",
		}}
	}
	checkTool := func(name string) (bool, string) {
		p, err := exec.LookPath(name)
		return err == nil, p
	}
	switch want {
	case "xdotool":
		if ok, p := checkTool("xdotool"); ok {
			return []Check{{Name: "meeting_detection_window", Status: "ok", Detail: "xdotool · " + p}}
		}
		return []Check{{Name: "meeting_detection_window", Status: "warn", Detail: "xdotool missing — run meetctl setup"}}
	case "wmctrl":
		if ok, p := checkTool("wmctrl"); ok {
			return []Check{{Name: "meeting_detection_window", Status: "ok", Detail: "wmctrl · " + p}}
		}
		return []Check{{Name: "meeting_detection_window", Status: "warn", Detail: "wmctrl missing — run meetctl setup"}}
	default:
		_, xErr := exec.LookPath("xdotool")
		_, wErr := exec.LookPath("wmctrl")
		if xErr == nil || wErr == nil {
			return []Check{{Name: "meeting_detection_window", Status: "ok", Detail: "xdotool or wmctrl available"}}
		}
		return []Check{{Name: "meeting_detection_window", Status: "warn", Detail: "install xdotool or wmctrl for window mode"}}
	}
}
