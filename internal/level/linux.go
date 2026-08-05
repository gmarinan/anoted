//go:build linux

package level

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"strconv"
	"sync"
)

const sampleRate = levelMeterSampleRate

type linuxMonitor struct {
	resolver    DeviceResolver
	mu          sync.Mutex
	system      float64
	mic         float64
	systemBands []float64
	micBands    []float64
	systemPrev  []float64
	micPrev     []float64

	latencyMsec     int
	processTimeMsec int

	systemCancel context.CancelFunc
	micCancel    context.CancelFunc
}

func newMonitor(resolver DeviceResolver) Monitor {
	def := DefaultStreamOptions()
	return &linuxMonitor{
		resolver:        resolver,
		latencyMsec:     def.LatencyMsec,
		processTimeMsec: def.ProcessTimeMsec,
	}
}

// DefaultStreamOptions returns parec timing defaults for level monitoring.
func DefaultStreamOptions() StreamOptions {
	return StreamOptions{LatencyMsec: 50, ProcessTimeMsec: 20}
}

// StreamOptions tunes parec chunk timing on Linux.
type StreamOptions struct {
	LatencyMsec     int
	ProcessTimeMsec int
}

func (m *linuxMonitor) SetStreamOptions(latencyMsec, processTimeMsec int) {
	if latencyMsec <= 0 {
		latencyMsec = DefaultStreamOptions().LatencyMsec
	}
	if processTimeMsec <= 0 {
		processTimeMsec = DefaultStreamOptions().ProcessTimeMsec
	}
	if processTimeMsec > latencyMsec {
		processTimeMsec = latencyMsec
	}
	m.mu.Lock()
	m.latencyMsec = latencyMsec
	m.processTimeMsec = processTimeMsec
	m.mu.Unlock()
}

func (m *linuxMonitor) Available() bool {
	_, err := exec.LookPath("parec")
	return err == nil
}

func (m *linuxMonitor) StartSystem(monitorID string) error {
	device, err := m.resolveSystem(monitorID)
	if err != nil {
		return err
	}
	m.StopSystem()
	cancel, err := m.startStream(device, func(buf []byte, peak float64, bands []float64, prev *[]float64) {
		m.mu.Lock()
		m.system = peak
		emph := emphasizeTransients(*prev, bands)
		*prev = bands
		m.systemBands = smoothBands(m.systemBands, emph)
		m.mu.Unlock()
	}, &m.systemPrev)
	if err != nil {
		return fmt.Errorf("start system level monitor: %w", err)
	}
	m.systemCancel = cancel
	return nil
}

func (m *linuxMonitor) StartMic(sourceID string) error {
	device, err := m.resolveMic(sourceID)
	if err != nil {
		return err
	}
	m.StopMic()
	cancel, err := m.startStream(device, func(buf []byte, peak float64, bands []float64, prev *[]float64) {
		m.mu.Lock()
		m.mic = peak
		emph := emphasizeTransients(*prev, bands)
		*prev = bands
		m.micBands = smoothBands(m.micBands, emph)
		m.mu.Unlock()
	}, &m.micPrev)
	if err != nil {
		return fmt.Errorf("start mic level monitor: %w", err)
	}
	m.micCancel = cancel
	return nil
}

func (m *linuxMonitor) StopSystem() error {
	if m.systemCancel != nil {
		m.systemCancel()
		m.systemCancel = nil
	}
	m.mu.Lock()
	m.system = 0
	m.systemBands = nil
	m.systemPrev = nil
	m.mu.Unlock()
	return nil
}

func (m *linuxMonitor) StopMic() error {
	if m.micCancel != nil {
		m.micCancel()
		m.micCancel = nil
	}
	m.mu.Lock()
	m.mic = 0
	m.micBands = nil
	m.micPrev = nil
	m.mu.Unlock()
	return nil
}

func (m *linuxMonitor) Read() LevelSnapshot {
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

func (m *linuxMonitor) Close() error {
	_ = m.StopSystem()
	_ = m.StopMic()
	return nil
}

func (m *linuxMonitor) resolveSystem(monitorID string) (string, error) {
	if m.resolver == nil {
		return monitorID, nil
	}
	sys, _, err := m.resolver.Resolve(monitorID, "")
	if err != nil {
		return "", err
	}
	return sys, nil
}

func (m *linuxMonitor) resolveMic(sourceID string) (string, error) {
	if m.resolver == nil {
		return sourceID, nil
	}
	_, mic, err := m.resolver.Resolve("", sourceID)
	if err != nil {
		return "", err
	}
	return mic, nil
}

func (m *linuxMonitor) startStream(device string, onChunk func(buf []byte, peak float64, bands []float64, prev *[]float64), prev *[]float64) (context.CancelFunc, error) {
	path, err := exec.LookPath("parec")
	if err != nil {
		return nil, fmt.Errorf("parec not found: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	m.mu.Lock()
	latency := m.latencyMsec
	process := m.processTimeMsec
	m.mu.Unlock()
	cmd := exec.CommandContext(ctx, path,
		"--record", "-d", device,
		"--format=s16le",
		"--rate="+strconv.Itoa(sampleRate),
		"--channels=1",
		"--latency-msec="+strconv.Itoa(latency),
		"--process-time-msec="+strconv.Itoa(process),
	)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		cancel()
		return nil, err
	}

	go func() {
		defer cancel()
		buf := make([]byte, chunkBytes)
		var smoothed float64
		for {
			n, err := io.ReadFull(stdout, buf)
			if err != nil {
				_ = cmd.Wait()
				return
			}
			chunk := buf[:n]
			sample := peakS16LE(chunk)
			smoothed = smoothPeak(smoothed, sample)
			bands := bandsFromPCM(chunk)
			onChunk(chunk, smoothed, bands, prev)
		}
	}()

	return cancel, nil
}
