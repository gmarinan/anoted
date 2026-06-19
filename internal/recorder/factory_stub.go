//go:build !linux && !windows

package recorder

import (
	"meetctl/internal/config"
	"meetctl/internal/platform"
)

func newPlatformRecorder(_ config.Config, _ platform.Info) Recorder {
	return NewDummyRecorder()
}
