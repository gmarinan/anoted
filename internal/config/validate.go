package config

import (
	"errors"
	"fmt"
	"strings"
)

// Validate reports configuration values that are out of range or unusable.
//
// applyDefaults only fills in zero values, and neither Load nor SaveRaw checked
// anything semantic, so poll_interval_ms: 1 (a pactl fork every millisecond),
// sample_rate: 3 or channels: 7 were all accepted and surfaced much later as a
// cryptic ffmpeg failure — usually once a meeting had already started.
func (c Config) Validate() error {
	var problems []string

	add := func(format string, args ...any) {
		problems = append(problems, fmt.Sprintf(format, args...))
	}

	if c.Audio.SampleRate != 0 && (c.Audio.SampleRate < 8000 || c.Audio.SampleRate > 192000) {
		add("audio.sample_rate: %d is outside 8000-192000", c.Audio.SampleRate)
	}
	if c.Audio.Channels != 0 && (c.Audio.Channels < 1 || c.Audio.Channels > 2) {
		add("audio.channels: %d must be 1 or 2", c.Audio.Channels)
	}
	if c.Audio.LevelUITickMS != 0 && c.Audio.LevelUITickMS < 10 {
		add("audio.level_ui_tick_ms: %d is below the 10ms floor", c.Audio.LevelUITickMS)
	}
	if c.Audio.LevelLatencyMsec < 0 {
		add("audio.level_latency_msec: %d cannot be negative", c.Audio.LevelLatencyMsec)
	}
	if c.Audio.LevelProcessTimeMsec < 0 {
		add("audio.level_process_time_msec: %d cannot be negative", c.Audio.LevelProcessTimeMsec)
	}

	if c.Detection.PollIntervalMS != 0 && c.Detection.PollIntervalMS < 250 {
		add("detection.poll_interval_ms: %d is below the 250ms floor "+
			"(each poll forks an external tool)", c.Detection.PollIntervalMS)
	}
	if c.Detection.MeetingEndGraceMS < 0 {
		add("detection.meeting_end_grace_ms: %d cannot be negative", c.Detection.MeetingEndGraceMS)
	}
	if mode := c.Detection.Mode; mode != "" && !validDetectionMode(mode) {
		add("detection.mode: %q must be one of mic, window, both, none", mode)
	}
	if tool := c.Detection.WindowTool; tool != "" && !validWindowTool(tool) {
		add("detection.window_tool: %q must be one of auto, xdotool, wmctrl, none", tool)
	}
	for provider, pc := range c.Detection.Providers {
		for _, p := range pc.Patterns {
			if strings.TrimSpace(p) == "" {
				// A blank pattern matches every window title, which with
				// auto-record enabled means recording continuously.
				add("detection.providers.%s: contains a blank pattern, which matches everything", provider)
				break
			}
		}
	}

	if c.OutputDir == "" {
		add("output_dir: must not be empty")
	}

	if len(problems) == 0 {
		return nil
	}
	return errors.New("invalid config:\n  - " + strings.Join(problems, "\n  - "))
}

func validDetectionMode(mode string) bool {
	switch mode {
	case "mic", "window", "both", "none":
		return true
	}
	return false
}

func validWindowTool(tool string) bool {
	switch tool {
	case "auto", "xdotool", "wmctrl", "none":
		return true
	}
	return false
}
