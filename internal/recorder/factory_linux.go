//go:build linux

package recorder

import (
	"anoted/internal/config"
	"anoted/internal/platform"
)

func newPlatformRecorder(cfg config.Config, plat platform.Info) Recorder {
	if plat.IsWSL2 {
		return NewWSLHelperRecorder(cfg)
	}
	for _, name := range cfg.Audio.LinuxBackendPriority {
		switch name {
		case "pipewire":
			if r, ok := NewLinuxPipeWireRecorder(cfg); ok {
				return r
			}
		case "pulseaudio", "ffmpeg":
			if r, ok := NewLinuxFFmpegRecorder(cfg); ok {
				return r
			}
		}
	}
	return NewDummyRecorder()
}
