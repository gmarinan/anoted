//go:build windows

package doctor

import (
	"fmt"
	"os/exec"

	"anoted/internal/audio"
	"anoted/internal/config"
	"anoted/internal/platform"
	"anoted/internal/recorder"
	"anoted/internal/wasapi"
)

func audioDeviceChecks(cfg config.Config) []Check {
	catalog, _ := audio.NewProvider().List()
	monitor, mic, err := recorder.ListAudioDevices(cfg)
	if err != nil {
		return []Check{{Name: "audio_devices", Status: "fail", Detail: err.Error()}}
	}
	checks := []Check{
		{Name: "system_monitor", Status: "ok", Detail: friendlyDeviceName(monitor, catalog, false)},
		{Name: "microphone", Status: "ok", Detail: friendlyDeviceName(mic, catalog, true)},
	}
	if cfg.Audio.SystemMonitor != "" && cfg.Audio.SystemMonitor != monitor {
		checks = append(checks, Check{
			Name:   "configured_system_monitor",
			Status: "ok",
			Detail: friendlyDeviceName(cfg.Audio.SystemMonitor, catalog, false),
		})
	}
	if cfg.Audio.Microphone != "" && cfg.Audio.Microphone != mic {
		checks = append(checks, Check{
			Name:   "configured_microphone",
			Status: "ok",
			Detail: friendlyDeviceName(cfg.Audio.Microphone, catalog, true),
		})
	}
	checks = append(checks, Check{
		Name:   "windows_level_meter",
		Status: "ok",
		Detail: "Level meters update during recording only",
	})
	checks = append(checks, Check{
		Name:   "windows_communications_ducking",
		Status: "warn",
		Detail: `Communications ducking: set to "Do nothing"`,
	})
	checks = append(checks, Check{
		Name:   "windows_output_format",
		Status: "warn",
		Detail: "Output format: prefer 48000 Hz",
	})
	return checks
}

func friendlyDeviceName(id string, catalog audio.Catalog, isMic bool) string {
	if id == "" {
		return "(auto)"
	}
	devices := catalog.Outputs
	if isMic {
		devices = catalog.Microphones
	}
	for _, d := range devices {
		if d.ID == id {
			return d.Name
		}
	}
	return wasapi.ShortLabel(id)
}

func detectionChecks(_ platform.Info, cfg config.Config) []Check {
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
		sessions, err := audio.ListCaptureSessions()
		if err != nil {
			checks = append(checks, Check{
				Name:   "meeting_detection_mic",
				Status: "fail",
				Detail: err.Error(),
			})
		} else {
			checks = append(checks, Check{
				Name:   "meeting_detection_mic",
				Status: "ok",
				Detail: fmt.Sprintf("WASAPI capture sessions (%d active)", len(sessions)),
			})
		}
	}

	if mode == "window" || mode == "both" {
		if _, err := exec.LookPath("powershell"); err != nil {
			checks = append(checks, Check{
				Name:   "meeting_detection_window",
				Status: "fail",
				Detail: "powershell not found",
			})
		} else if _, err := exec.LookPath("tasklist"); err != nil {
			checks = append(checks, Check{
				Name:   "meeting_detection_window",
				Status: "warn",
				Detail: "tasklist not found",
			})
		} else {
			checks = append(checks, Check{
				Name:   "meeting_detection_window",
				Status: "ok",
				Detail: "window titles via PowerShell + tasklist",
			})
		}
	}

	return checks
}
