//go:build !linux && !windows

package detector

import "meetctl/internal/platform"

func newPlatformDetector(cfg Config, _ platform.Info) Detector {
	return NewMockDetector()
}
