//go:build !linux && !windows

package recorder

import "anoted/internal/config"

func resolveAudioDevices(_ config.Config) (systemMonitor, microphone string, err error) {
	return "", "", nil
}

func ListAudioDevices(_ config.Config) (monitor, mic string, err error) {
	return "", "", nil
}
