//go:build windows

package wasapi

import (
	"fmt"
	"sync"
	"unsafe"

	"github.com/gen2brain/malgo"
)

const streamChunkSize = 8192

// Stream captures PCM from a single WASAPI endpoint.
type Stream struct {
	device             *malgo.Device
	ch                 chan []byte
	mu                 sync.Mutex
	closed             bool
	pending            []byte
	wake               chan struct{}
	done               chan struct{}
	wg                 sync.WaitGroup
	configuredRate     uint32
	configuredChannels int
}

// LoopbackStreamConfig configures system audio loopback capture.
type LoopbackStreamConfig struct {
	DeviceID   malgo.DeviceID
	SampleRate uint32
	Channels   uint32
}

// CaptureStreamConfig configures microphone capture.
type CaptureStreamConfig struct {
	DeviceID   malgo.DeviceID
	SampleRate uint32
	Channels   uint32
}

// StartLoopback opens a loopback capture stream.
func StartLoopback(cfg LoopbackStreamConfig) (*Stream, error) {
	cfg.SampleRate, cfg.Channels = CanonicalFormat(int(cfg.SampleRate), int(cfg.Channels))
	ctx, err := Context()
	if err != nil {
		return nil, err
	}
	s := &Stream{
		ch:                 make(chan []byte, 16),
		wake:               make(chan struct{}, 1),
		done:               make(chan struct{}),
		configuredRate:     cfg.SampleRate,
		configuredChannels: int(cfg.Channels),
	}

	deviceCfg := malgo.DefaultDeviceConfig(malgo.Loopback)
	deviceCfg.SampleRate = cfg.SampleRate
	deviceCfg.Capture.Format = malgo.FormatS16
	deviceCfg.Capture.Channels = cfg.Channels
	deviceCfg.Playback.DeviceID = malgoDevicePtr(cfg.DeviceID)
	applyDeviceConfig(&deviceCfg)

	onData := func(_, input []byte, frameCount uint32) {
		if len(input) == 0 {
			return
		}
		s.push(input)
	}

	dev, err := malgo.InitDevice(ctx.Context, deviceCfg, malgo.DeviceCallbacks{Data: onData})
	if err != nil {
		return nil, fmt.Errorf("init loopback device: %w", err)
	}
	s.device = dev
	s.wg.Add(1)
	go s.forward()
	if err := dev.Start(); err != nil {
		s.stopForward()
		dev.Uninit()
		return nil, fmt.Errorf("start loopback device: %w", err)
	}
	return s, nil
}

// StartCapture opens a microphone capture stream.
func StartCapture(cfg CaptureStreamConfig) (*Stream, error) {
	cfg.SampleRate, cfg.Channels = CanonicalFormat(int(cfg.SampleRate), int(cfg.Channels))
	ctx, err := Context()
	if err != nil {
		return nil, err
	}
	s := &Stream{
		ch:                 make(chan []byte, 16),
		wake:               make(chan struct{}, 1),
		done:               make(chan struct{}),
		configuredRate:     cfg.SampleRate,
		configuredChannels: int(cfg.Channels),
	}

	deviceCfg := malgo.DefaultDeviceConfig(malgo.Capture)
	deviceCfg.SampleRate = cfg.SampleRate
	deviceCfg.Capture.Format = malgo.FormatS16
	deviceCfg.Capture.Channels = cfg.Channels
	deviceCfg.Capture.DeviceID = malgoDevicePtr(cfg.DeviceID)
	applyDeviceConfig(&deviceCfg)

	onData := func(_, input []byte, frameCount uint32) {
		if len(input) == 0 {
			return
		}
		s.push(input)
	}

	dev, err := malgo.InitDevice(ctx.Context, deviceCfg, malgo.DeviceCallbacks{Data: onData})
	if err != nil {
		return nil, fmt.Errorf("init capture device: %w", err)
	}
	s.device = dev
	s.wg.Add(1)
	go s.forward()
	if err := dev.Start(); err != nil {
		s.stopForward()
		dev.Uninit()
		return nil, fmt.Errorf("start capture device: %w", err)
	}
	return s, nil
}

func applyDeviceConfig(cfg *malgo.DeviceConfig) {
	cfg.Wasapi.NoAutoStreamRouting = 1
	cfg.Playback.ShareMode = malgo.Shared
	cfg.Capture.ShareMode = malgo.Shared
	cfg.Resampling.Algorithm = malgo.ResampleAlgorithmLinear
	cfg.Resampling.Linear.LpfOrder = 8
}

func malgoDevicePtr(id malgo.DeviceID) unsafe.Pointer {
	if id == (malgo.DeviceID{}) {
		return nil
	}
	return id.Pointer()
}

// Ch returns the PCM chunk channel.
func (s *Stream) Ch() <-chan []byte {
	return s.ch
}

// SampleRate returns the configured capture sample rate of delivered PCM.
func (s *Stream) SampleRate() uint32 {
	return s.configuredRate
}

// Channels returns the configured capture channel count of delivered PCM.
func (s *Stream) Channels() int {
	if s.configuredChannels > 0 {
		return s.configuredChannels
	}
	return CanonicalChannels
}

// InternalSampleRate returns the native rate negotiated with WASAPI.
func (s *Stream) InternalSampleRate() uint32 {
	if s.device == nil {
		return 0
	}
	if rate := s.device.CaptureInternalSampleRate(); rate > 0 {
		return rate
	}
	return s.device.SampleRate()
}

// InternalChannels returns the native channel count negotiated with WASAPI.
func (s *Stream) InternalChannels() uint32 {
	if s.device == nil {
		return 0
	}
	if ch := s.device.CaptureInternalChannels(); ch > 0 {
		return ch
	}
	return uint32(s.Channels())
}

// Stop closes the stream.
func (s *Stream) Stop() error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil
	}
	s.closed = true
	dev := s.device
	s.device = nil
	s.mu.Unlock()

	if dev != nil {
		_ = dev.Stop()
		dev.Uninit()
	}
	s.stopForward()
	s.wg.Wait()
	return nil
}

func (s *Stream) stopForward() {
	select {
	case <-s.done:
	default:
		close(s.done)
	}
}

func (s *Stream) push(input []byte) {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return
	}
	s.pending = append(s.pending, input...)
	s.mu.Unlock()
	select {
	case s.wake <- struct{}{}:
	default:
	}
}

func (s *Stream) forward() {
	defer s.wg.Done()
	defer close(s.ch)
	for {
		if !s.emitPending() {
			select {
			case <-s.done:
				s.drainAll()
				return
			case <-s.wake:
			}
		}
	}
}

func (s *Stream) emitPending() bool {
	s.mu.Lock()
	if len(s.pending) == 0 {
		s.mu.Unlock()
		return false
	}
	n := len(s.pending)
	if n > streamChunkSize {
		n = streamChunkSize
	}
	chunk := make([]byte, n)
	copy(chunk, s.pending[:n])
	s.pending = s.pending[n:]
	s.mu.Unlock()
	s.ch <- chunk
	return true
}

func (s *Stream) drainAll() {
	for {
		s.mu.Lock()
		if len(s.pending) == 0 {
			s.mu.Unlock()
			return
		}
		chunk := append([]byte(nil), s.pending...)
		s.pending = nil
		s.mu.Unlock()
		select {
		case s.ch <- chunk:
		case <-s.done:
			return
		}
	}
}
