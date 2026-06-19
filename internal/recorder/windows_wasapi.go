//go:build windows

package recorder

import (
	"meetctl/internal/config"
)

// WindowsWASAPIRecorder is a placeholder for native WASAPI capture.
// Real implementation will use Core Audio APIs or the windows-recorder helper.
type WindowsWASAPIRecorder struct {
	inner *DummyRecorder
}

func NewWindowsWASAPIRecorder(_ config.Config) (*WindowsWASAPIRecorder, bool) {
	// MVP: WASAPI not yet implemented; factory falls through to dummy.
	return nil, false
}
