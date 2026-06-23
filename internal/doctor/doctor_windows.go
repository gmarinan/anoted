package doctor

import (
	"fmt"
	"os/exec"

	"anoted/internal/audio"
	"anoted/internal/config"
	"anoted/internal/platform"
	"anoted/internal/recorder"
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
	checks = append(checks, Check{
		Name:   "windows_level_meter",
		Status: "ok",
		Detail: "System/mic level meters update during recording only (no idle loopback)",
	})
	checks = append(checks, Check{
		Name:   "windows_communications_ducking",
		Status: "warn",
		Detail: "Set Sound > Communications to \"Do nothing\" so meeting apps do not duck other audio by 80%",
	})
	checks = append(checks, Check{
		Name:   "windows_output_format",
		Status: "warn",
		Detail: "Use a 48000 Hz shared output format for the default playback device (Sound > device Properties > Advanced)",
	})
	return checks
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
