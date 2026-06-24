//go:build !linux && !windows

package tray

func newPlatformIndicator(opts Options) Indicator {
	return noopIndicator{}
}
