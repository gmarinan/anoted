//go:build windows

package recorder

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"anoted/internal/config"
	"anoted/internal/session"
	"anoted/internal/wasapi"
)

// WindowsWASAPIRecorder captures system loopback and microphone via WASAPI.
type WindowsWASAPIRecorder struct {
	cfg    config.Config
	status RecorderStatus
	mu     sync.Mutex

	dual   *wasapi.DualRecorder
	writer *WAVWriter
}

func NewWindowsWASAPIRecorder(cfg config.Config) (*WindowsWASAPIRecorder, bool) {
	if _, err := wasapi.Context(); err != nil {
		return nil, false
	}
	return &WindowsWASAPIRecorder{
		cfg:    cfg,
		status: RecorderStatus{Status: StatusIdle, Backend: "wasapi"},
	}, true
}

func (r *WindowsWASAPIRecorder) Name() string { return "wasapi" }

func (r *WindowsWASAPIRecorder) Start(_ context.Context, sess SessionConfig) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.status.Status == StatusRecording {
		return fmt.Errorf("already recording")
	}

	dir := sessionDir(sess)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create session dir: %w", err)
	}

	devs, err := resolveFromSession(sess)
	if err != nil {
		return err
	}

	loopID, err := wasapi.ParseLoopbackID(devs.system)
	if err != nil {
		return err
	}
	capID, err := wasapi.ParseCaptureID(devs.mic)
	if err != nil {
		return err
	}

	rate := sess.SampleRate
	if rate <= 0 {
		rate = 48000
	}
	channels := sess.Channels
	if channels <= 0 {
		channels = 2
	}

	r.writer = NewWAVWriter(rate, channels)
	dual, err := wasapi.StartDualRecorder(wasapi.DualRecorderConfig{
		LoopbackID: loopID,
		CaptureID:  capID,
		SampleRate: uint32(rate),
		Channels:   uint32(channels),
		OnPCM: func(pcm []byte) {
			r.mu.Lock()
			defer r.mu.Unlock()
			if r.writer != nil {
				r.writer.WritePCM(pcm)
			}
		},
	})
	if err != nil {
		return fmt.Errorf("start wasapi capture: %w", err)
	}
	r.dual = dual

	started := time.Now()
	if err := session.WriteMetadataFile(dir, session.Metadata{
		StartedAt:    started,
		Provider:     sess.Provider,
		Platform:     sess.Platform,
		Backend:      r.Name(),
		SystemDevice: devs.system,
		MicDevice:    devs.mic,
		AutoRecord:   sess.AutoRecord,
		Manual:       sess.Manual,
	}); err != nil {
		r.dual.Stop()
		r.dual = nil
		return err
	}

	r.status = RecorderStatus{
		Status:     StatusRecording,
		Backend:    r.Name(),
		SessionDir: dir,
		StartedAt:  started,
	}
	return nil
}

func (r *WindowsWASAPIRecorder) Stop(_ context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.status.Status != StatusRecording {
		return nil
	}

	if r.dual != nil {
		r.dual.Stop()
		r.dual = nil
	}

	if r.writer != nil && r.status.SessionDir != "" {
		path := dirFile(r.status.SessionDir, SessionAudioFile)
		if err := os.WriteFile(path, r.writer.Bytes(), 0o644); err != nil {
			r.status.Status = StatusError
			r.status.Error = err.Error()
			return fmt.Errorf("write %s: %w", SessionAudioFile, err)
		}
		r.writer = nil
	}

	ended := time.Now()
	meta := session.Metadata{
		StartedAt: r.status.StartedAt,
		EndedAt:   ended,
		Duration:  ended.Sub(r.status.StartedAt).Round(time.Second).String(),
		Backend:   r.Name(),
	}
	if err := session.WriteMetadataFile(r.status.SessionDir, meta); err != nil {
		r.status.Status = StatusError
		r.status.Error = err.Error()
		return err
	}

	r.status.Status = StatusIdle
	r.status.StartedAt = time.Time{}
	return nil
}

func (r *WindowsWASAPIRecorder) Status() RecorderStatus {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.status
}
