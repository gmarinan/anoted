//go:build !windows

package audio

import (
	"fmt"
	"runtime"
)

// ListCaptureSessions returns active microphone capture sessions.
func ListCaptureSessions() ([]CaptureSession, error) {
	return nil, fmt.Errorf("capture sessions: not supported on %s", runtime.GOOS)
}
