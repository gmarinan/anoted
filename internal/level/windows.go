//go:build windows

package level

import (
	"sync"

	"anoted/internal/wasapi"
)

type windowsMonitor struct {
	resolver    DeviceResolver
	mu          sync.Mutex
	system      float64
	mic         float64
	systemBands []float64
	micBands    []float64

	feedChannels int
}

func newMonitor(resolver DeviceResolver) Monitor {
	return &windowsMonitor{resolver: resolver}
}

func (m *windowsMonitor) SetStreamOptions(_, _ int) {}

func (m *windowsMonitor) Available() bool {
	_, err := wasapi.Context()
	return err == nil
}

func (m *windowsMonitor) StartSystem(monitorID string) error {
	if _, err := m.resolveSystem(monitorID); err != nil {
		return err
	}
	m.StopSystem()
	// Opening a loopback client for idle level monitoring reconfigures the Windows
	// audio engine and can silence normal playback. System levels are fed from the
	// recorder via FeedSystemPCM while recording.
	return nil
}

func (m *windowsMonitor) StartMic(sourceID string) error {
	if _, err := m.resolveMic(sourceID); err != nil {
		return err
	}
	m.StopMic()
	// Mic levels while recording come from the recorder via FeedMicPCM.
	return nil
}

func (m *windowsMonitor) FeedSystemPCM(pcm []byte) {
	m.feedPCM(pcm, true)
}

func (m *windowsMonitor) FeedMicPCM(pcm []byte) {
	m.feedPCM(pcm, false)
}

func (m *windowsMonitor) SetFeedChannels(channels int) {
	if channels <= 0 {
		channels = 2
	}
	m.mu.Lock()
	m.feedChannels = channels
	m.mu.Unlock()
}

func (m *windowsMonitor) feedPCM(pcm []byte, system bool) {
	if len(pcm) < 2 {
		return
	}
	channels := m.feedChannels
	if channels <= 0 {
		channels = 2
	}
	mono := wasapi.DownmixToMono(nil, pcm, channels)
	sample := peakS16LE(mono)
	m.mu.Lock()
	defer m.mu.Unlock()
	if system {
		m.system = smoothPeak(m.system, sample)
		m.systemBands = peakBands(m.systemBands, sample)
	} else {
		m.mic = smoothPeak(m.mic, sample)
		m.micBands = peakBands(m.micBands, sample)
	}
}

func (m *windowsMonitor) StopSystem() error {
	m.mu.Lock()
	m.system = 0
	m.systemBands = nil
	m.feedChannels = 0
	m.mu.Unlock()
	return nil
}

func (m *windowsMonitor) StopMic() error {
	m.mu.Lock()
	m.mic = 0
	m.micBands = nil
	m.mu.Unlock()
	return nil
}

func (m *windowsMonitor) Read() LevelSnapshot {
	m.mu.Lock()
	defer m.mu.Unlock()
	return LevelSnapshot{
		System:      m.system,
		Mic:         m.mic,
		SystemBands: copyBands(m.systemBands),
		MicBands:    copyBands(m.micBands),
	}
}

func copyBands(in []float64) []float64 {
	if len(in) == 0 {
		return nil
	}
	out := make([]float64, len(in))
	copy(out, in)
	return out
}

func (m *windowsMonitor) Close() error {
	_ = m.StopSystem()
	_ = m.StopMic()
	return nil
}

func (m *windowsMonitor) resolveSystem(monitorID string) (string, error) {
	if m.resolver == nil {
		return monitorID, nil
	}
	sys, _, err := m.resolver.Resolve(monitorID, "")
	if err != nil {
		return "", err
	}
	return sys, nil
}

func (m *windowsMonitor) resolveMic(sourceID string) (string, error) {
	if m.resolver == nil {
		return sourceID, nil
	}
	_, mic, err := m.resolver.Resolve("", sourceID)
	if err != nil {
		return "", err
	}
	return mic, nil
}

// LiveWhenIdle is false: StartSystem and StartMic are deliberate no-ops here
// (opening a loopback client reconfigures the Windows audio engine), so bands
// only arrive via FeedSystemPCM/FeedMicPCM during a recording.
func (m *windowsMonitor) LiveWhenIdle() bool { return false }
