//go:build !linux && !windows

package recorder

import (
	"anoted/internal/config"
	"anoted/internal/platform"
)

func newPlatformRecorder(_ config.Config, _ platform.Info) Recorder {
	return NewDummyRecorder()
}
