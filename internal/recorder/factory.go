package recorder

import (
	"anoted/internal/config"
	"anoted/internal/platform"
)

// New selects the best available recorder for the platform.
func New(cfg config.Config, plat platform.Info, forceDummy bool) Recorder {
	if forceDummy {
		return NewDummyRecorder()
	}
	return newPlatformRecorder(cfg, plat)
}
