//go:build windows

package wasapi

import (
	"context"
	"sync"

	"github.com/gen2brain/malgo"
)

// DualRecorder mixes loopback and microphone into a PCM sink.
type DualRecorder struct {
	loop   *Stream
	mic    *Stream
	cancel context.CancelFunc
	wg     sync.WaitGroup
	onPCM  func([]byte)
}

// DualRecorderConfig configures dual capture.
type DualRecorderConfig struct {
	LoopbackID malgo.DeviceID
	CaptureID  malgo.DeviceID
	SampleRate uint32
	Channels   uint32
	OnPCM      func([]byte)
}

// StartDualRecorder begins loopback + mic capture and invokes onPCM with mixed s16le frames.
func StartDualRecorder(cfg DualRecorderConfig) (*DualRecorder, error) {
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
		loop:   loop,
		mic:    mic,
		cancel: cancel,
		onPCM:  cfg.OnPCM,
	}
	r.wg.Add(1)
	go r.run(ctx)
	return r, nil
}

func (r *DualRecorder) run(ctx context.Context) {
	defer r.wg.Done()
	var pendingA, pendingB []byte
	var mixBuf []byte
	for {
		select {
		case <-ctx.Done():
			return
		case chunk, ok := <-r.loop.Ch():
			if !ok {
				return
			}
			pendingA = append(pendingA, chunk...)
			mixBuf, pendingA, pendingB = r.drain(pendingA, pendingB, mixBuf)
		case chunk, ok := <-r.mic.Ch():
			if !ok {
				return
			}
			pendingB = append(pendingB, chunk...)
			mixBuf, pendingA, pendingB = r.drain(pendingA, pendingB, mixBuf)
		}
	}
}

func (r *DualRecorder) drain(a, b, mixBuf []byte) ([]byte, []byte, []byte) {
	frameBytes := 4 // stereo s16
	for len(a) >= frameBytes || len(b) >= frameBytes {
		var fa, fb []byte
		if len(a) >= frameBytes {
			fa = a[:frameBytes]
			a = a[frameBytes:]
		}
		if len(b) >= frameBytes {
			fb = b[:frameBytes]
			b = b[frameBytes:]
		}
		mixBuf = MixS16(mixBuf[:0], fa, fb)
		r.onPCM(mixBuf)
	}
	return mixBuf, a, b
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
