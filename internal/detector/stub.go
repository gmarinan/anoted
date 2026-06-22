//go:build !linux && !windows

package detector

import "anoted/internal/platform"

func newPlatformDetector(cfg Config, _ platform.Info) Detector {
	return NewMockDetector()
}
