//go:build windows

package wasapi

import (
	"context"
	"sync"
	"time"

	"github.com/gen2brain/malgo"
)

// CaptureDiagnostics reports configured and native capture parameters.
type CaptureDiagnostics struct {
	ConfiguredRate       uint32
	ConfiguredChannels   uint32
	LoopInternalRate     uint32
	LoopInternalChannels uint32
	MicInternalRate      uint32
	MicInternalChannels  uint32
}

// DualRecorder mixes loopback and microphone into a PCM sink.
type DualRecorder struct {
	loop       *Stream
	mic        *Stream
	cancel     context.CancelFunc
	wg         sync.WaitGroup
	mixer      *MasterClockMixer
	sampleRate uint32
	channels   uint32
}

// DualRecorderConfig configures dual capture.
type DualRecorderConfig struct {
	LoopbackID malgo.DeviceID
	CaptureID  malgo.DeviceID
	SampleRate uint32
	Channels   uint32
	OnPCM      func([]byte)
	OnLoopPCM  func([]byte)
	OnMicPCM   func([]byte)
}

// StartDualRecorder begins loopback + mic capture and invokes onPCM with mixed s16le frames.
func StartDualRecorder(cfg DualRecorderConfig) (*DualRecorder, error) {
	cfg.SampleRate, cfg.Channels = CanonicalFormat(int(cfg.SampleRate), int(cfg.Channels))
	if cfg.OnPCM == nil {
		cfg.OnPCM = func([]byte) {}
	}
	loop, err := StartLoopback(LoopbackStreamConfig{
		DeviceID:   cfg.LoopbackID,
		SampleRate: cfg.SampleRate,
		Channels:   cfg.Channels,
	})
	if err != nil {
		return nil, err
	}
	mic, err := StartCapture(CaptureStreamConfig{
		DeviceID:   cfg.CaptureID,
		SampleRate: cfg.SampleRate,
		Channels:   cfg.Channels,
	})
	if err != nil {
		_ = loop.Stop()
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())
	r := &DualRecorder{
		loop:       loop,
		mic:        mic,
		cancel:     cancel,
		sampleRate: cfg.SampleRate,
		channels:   cfg.Channels,
		mixer: NewMasterClockMixer(int(cfg.SampleRate), int(cfg.Channels), func(pcm []byte) {
			cfg.OnPCM(pcm)
		}),
	}
	r.wg.Add(1)
	go r.run(ctx, cfg.OnLoopPCM, cfg.OnMicPCM)
	return r, nil
}

func (r *DualRecorder) run(ctx context.Context, onLoopPCM, onMicPCM func([]byte)) {
	defer r.wg.Done()
	ticker := time.NewTicker(r.mixer.TickInterval())
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.mixer.EmitTicks()
		case chunk, ok := <-r.loop.Ch():
			if !ok {
				return
			}
			if onLoopPCM != nil {
				onLoopPCM(chunk)
			}
			r.mixer.PushLoop(chunk)
		case chunk, ok := <-r.mic.Ch():
			if !ok {
				return
			}
			if onMicPCM != nil {
				onMicPCM(chunk)
			}
			r.mixer.PushMic(chunk)
		}
	}
}

// SampleRate returns the configured capture sample rate.
func (r *DualRecorder) SampleRate() uint32 {
	if r.sampleRate > 0 {
		return r.sampleRate
	}
	if r.loop != nil {
		return r.loop.SampleRate()
	}
	return CanonicalSampleRate
}

// Channels returns the configured capture channel count.
func (r *DualRecorder) Channels() uint32 {
	if r.channels > 0 {
		return r.channels
	}
	return CanonicalChannels
}

// Diagnostics returns configured and native capture parameters.
func (r *DualRecorder) Diagnostics() CaptureDiagnostics {
	d := CaptureDiagnostics{
		ConfiguredRate:     r.SampleRate(),
		ConfiguredChannels: r.Channels(),
	}
	if r.loop != nil {
		d.LoopInternalRate = r.loop.InternalSampleRate()
		d.LoopInternalChannels = uint32(r.loop.InternalChannels())
	}
	if r.mic != nil {
		d.MicInternalRate = r.mic.InternalSampleRate()
		d.MicInternalChannels = uint32(r.mic.InternalChannels())
	}
	return d
}

// Stop ends capture and waits for the mixer goroutine.
func (r *DualRecorder) Stop() {
	if r.cancel != nil {
		r.cancel()
	}
	if r.loop != nil {
		_ = r.loop.Stop()
	}
	if r.mic != nil {
		_ = r.mic.Stop()
	}
	r.wg.Wait()
}
