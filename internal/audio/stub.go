//go:build !linux

package audio

import "fmt"

type stubProvider struct{}

func newProvider() Provider { return stubProvider{} }

func (stubProvider) List() (Catalog, error) {
	return Catalog{}, nil
}

func (stubProvider) Resolve(systemMonitor, microphone string) (string, string, error) {
	if systemMonitor != "" && microphone != "" {
		return systemMonitor, microphone, nil
	}
	return "", "", fmt.Errorf("audio device listing not available on this platform")
}

func (stubProvider) MonitorWarning(string) string { return "" }
