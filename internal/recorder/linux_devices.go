//go:build linux

package recorder

import (
	"meetctl/internal/audio"
	"meetctl/internal/config"
)

type resolvedDevices struct {
	system string
	mic    string
}

func resolveFromSession(sess SessionConfig) (resolvedDevices, error) {
	p := audio.NewProvider()
	sys, mic, err := p.Resolve(sess.SystemMonitor, sess.Microphone)
	if err != nil {
		return resolvedDevices{}, err
	}
	return resolvedDevices{system: sys, mic: mic}, nil
}

func resolveAudioDevices(cfg config.Config) (systemMonitor, microphone string, err error) {
	p := audio.NewProvider()
	return p.Resolve(cfg.Audio.SystemMonitor, cfg.Audio.Microphone)
}

// ListAudioDevices returns resolved monitor and mic for doctor output.
func ListAudioDevices(cfg config.Config) (monitor, mic string, err error) {
	return resolveAudioDevices(cfg)
}
