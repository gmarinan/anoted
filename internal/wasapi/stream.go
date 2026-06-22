//go:build windows

package wasapi

import (
	"fmt"
	"sync"
	"unsafe"

	"github.com/gen2brain/malgo"
)

// Stream captures PCM from a single WASAPI endpoint.
type Stream struct {
	device   *malgo.Device
	ch       chan []byte
	mu       sync.Mutex
	closed   bool
	bufPool  sync.Pool
	channels int
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
	if cfg.SampleRate == 0 {
		cfg.SampleRate = 48000
	}
	if cfg.Channels == 0 {
		cfg.Channels = 2
	}
	ctx, err := Context()
	if err != nil {
		return nil, err
	}
	s := &Stream{ch: make(chan []byte, 32), channels: int(cfg.Channels)}
	s.bufPool.New = func() any { return make([]byte, 4096) }

	deviceCfg := malgo.DefaultDeviceConfig(malgo.Loopback)
	deviceCfg.SampleRate = cfg.SampleRate
	deviceCfg.Capture.Format = malgo.FormatS16
	deviceCfg.Capture.Channels = cfg.Channels
	deviceCfg.Playback.DeviceID = malgoDevicePtr(cfg.DeviceID)

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
	if err := dev.Start(); err != nil {
		dev.Uninit()
		return nil, fmt.Errorf("start loopback device: %w", err)
	}
	s.device = dev
	return s, nil
}

// StartCapture opens a microphone capture stream.
func StartCapture(cfg CaptureStreamConfig) (*Stream, error) {
	if cfg.SampleRate == 0 {
		cfg.SampleRate = 48000
	}
	if cfg.Channels == 0 {
		cfg.Channels = 2
	}
	ctx, err := Context()
	if err != nil {
		return nil, err
	}
	s := &Stream{ch: make(chan []byte, 32), channels: int(cfg.Channels)}
	s.bufPool.New = func() any { return make([]byte, 4096) }

	deviceCfg := malgo.DefaultDeviceConfig(malgo.Capture)
	deviceCfg.SampleRate = cfg.SampleRate
	deviceCfg.Capture.Format = malgo.FormatS16
	deviceCfg.Capture.Channels = cfg.Channels
	deviceCfg.Capture.DeviceID = malgoDevicePtr(cfg.DeviceID)

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
	if err := dev.Start(); err != nil {
		dev.Uninit()
		return nil, fmt.Errorf("start capture device: %w", err)
	}
	s.device = dev
	return s, nil
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

// Stop closes the stream.
func (s *Stream) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}
	s.closed = true
	close(s.ch)
	if s.device != nil {
		_ = s.device.Stop()
		s.device.Uninit()
		s.device = nil
	}
	return nil
}

func (s *Stream) push(input []byte) {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return
	}
	s.mu.Unlock()

	buf := s.bufPool.Get().([]byte)
	if cap(buf) < len(input) {
		buf = make([]byte, len(input))
	}
	buf = buf[:len(input)]
	copy(buf, input)

	select {
	case s.ch <- buf:
	default:
		s.bufPool.Put(buf)
	}
}
