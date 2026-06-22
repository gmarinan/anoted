//go:build windows

package recorder

import (
	"anoted/internal/config"
	"anoted/internal/platform"
)

func newPlatformRecorder(cfg config.Config, _ platform.Info) Recorder {
	for _, name := range cfg.Audio.WindowsBackendPriority {
		if name == "wasapi" {
			if r, ok := NewWindowsWASAPIRecorder(cfg); ok {
				return r
			}
		}
	}
	return NewDummyRecorder()
}
