//go:build windows

package level

import (
	"context"
	"fmt"
	"sync"

	"anoted/internal/wasapi"
)

const (
	sampleRate   = 16000
	chunkSamples = 320
	chunkBytes   = chunkSamples * 2
)

type windowsMonitor struct {
	resolver DeviceResolver
	mu       sync.Mutex
	system   float64
	mic      float64
	systemBands []float64
	micBands    []float64
	systemPrev  []float64
	micPrev     []float64

	loopStream *wasapi.Stream
	micStream  *wasapi.Stream
	loopCancel context.CancelFunc
	micCancel  context.CancelFunc
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
	device, err := m.resolveSystem(monitorID)
	if err != nil {
		return err
	}
	m.StopSystem()

	loopID, err := wasapi.ParseLoopbackID(device)
	if err != nil {
		return err
	}
	stream, err := wasapi.StartLoopback(wasapi.LoopbackStreamConfig{
		DeviceID:   loopID,
		SampleRate: sampleRate,
		Channels:   1,
	})
	if err != nil {
		return fmt.Errorf("start system level monitor: %w", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	m.loopStream = stream
	m.loopCancel = cancel
	go m.consume(ctx, stream, true)
	return nil
}

func (m *windowsMonitor) StartMic(sourceID string) error {
	device, err := m.resolveMic(sourceID)
	if err != nil {
		return err
	}
	m.StopMic()

	capID, err := wasapi.ParseCaptureID(device)
	if err != nil {
		return err
	}
	stream, err := wasapi.StartCapture(wasapi.CaptureStreamConfig{
		DeviceID:   capID,
		SampleRate: sampleRate,
		Channels:   1,
	})
	if err != nil {
		return fmt.Errorf("start mic level monitor: %w", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	m.micStream = stream
	m.micCancel = cancel
	go m.consume(ctx, stream, false)
	return nil
}

func (m *windowsMonitor) consume(ctx context.Context, stream *wasapi.Stream, system bool) {
	var buf []byte
	for {
		select {
		case <-ctx.Done():
			return
		case chunk, ok := <-stream.Ch():
			if !ok {
				return
			}
			buf = append(buf, chunk...)
			for len(buf) >= chunkBytes {
				frame := buf[:chunkBytes]
				buf = buf[chunkBytes:]
				sample := peakS16LE(frame)
				bands := bandsFromPCM(frame)
				m.mu.Lock()
				if system {
					m.system = smoothPeak(m.system, sample)
					emph := emphasizeTransients(m.systemPrev, bands)
					m.systemPrev = bands
					m.systemBands = smoothBands(m.systemBands, emph)
				} else {
					m.mic = smoothPeak(m.mic, sample)
					emph := emphasizeTransients(m.micPrev, bands)
					m.micPrev = bands
					m.micBands = smoothBands(m.micBands, emph)
				}
				m.mu.Unlock()
			}
		}
	}
}

func (m *windowsMonitor) StopSystem() error {
	if m.loopCancel != nil {
		m.loopCancel()
		m.loopCancel = nil
	}
	if m.loopStream != nil {
		_ = m.loopStream.Stop()
		m.loopStream = nil
	}
	m.mu.Lock()
	m.system = 0
	m.systemBands = nil
	m.systemPrev = nil
	m.mu.Unlock()
	return nil
}

func (m *windowsMonitor) StopMic() error {
	if m.micCancel != nil {
		m.micCancel()
		m.micCancel = nil
	}
	if m.micStream != nil {
		_ = m.micStream.Stop()
		m.micStream = nil
	}
	m.mu.Lock()
	m.mic = 0
	m.micBands = nil
	m.micPrev = nil
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
