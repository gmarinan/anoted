//go:build windows

package audio

import (
	"fmt"

	"anoted/internal/wasapi"
)

type windowsProvider struct{}

func newProvider() Provider { return windowsProvider{} }

func (windowsProvider) List() (Catalog, error) {
	outputs, err := wasapi.ListLoopback()
	if err != nil {
		return Catalog{}, err
	}
	mics, err := wasapi.ListCapture()
	if err != nil {
		return Catalog{}, err
	}

	var cat Catalog
	cat.Outputs = append(cat.Outputs, Device{
		ID: AutoID, Name: AutoLabel, IsDefault: true,
	})
	cat.Microphones = append(cat.Microphones, Device{
		ID: AutoID, Name: AutoLabel, IsDefault: true,
	})

	for _, d := range outputs {
		cat.Outputs = append(cat.Outputs, Device{
			ID:        d.ID,
			Name:      d.Name,
			IsDefault: d.IsDefault,
			Format:    d.Format,
		})
	}
	for _, d := range mics {
		cat.Microphones = append(cat.Microphones, Device{
			ID:        d.ID,
			Name:      d.Name,
			IsDefault: d.IsDefault,
			Format:    d.Format,
		})
	}
	return cat, nil
}

func (windowsProvider) Resolve(systemMonitor, microphone string) (string, string, error) {
	loopID, err := wasapi.ResolveLoopback(systemMonitor)
	if err != nil {
		return "", "", fmt.Errorf("resolve system loopback: %w", err)
	}
	capID, err := wasapi.ResolveCapture(microphone)
	if err != nil {
		return "", "", fmt.Errorf("resolve microphone: %w", err)
	}
	if systemMonitor == "" {
		systemMonitor = wasapi.LoopbackID(loopID)
	}
	if microphone == "" {
		microphone = wasapi.CaptureID(capID)
	}
	return systemMonitor, microphone, nil
}

func (p windowsProvider) MonitorWarning(configuredMonitor string) string {
	if configuredMonitor == "" {
		return ""
	}
	devs, err := wasapi.ListLoopback()
	if err != nil {
		return ""
	}
	for _, d := range devs {
		if d.ID == configuredMonitor {
			if !d.IsDefault {
				return "configured output device is not the system default"
			}
			return ""
		}
	}
	return "configured output device was not found"
}
