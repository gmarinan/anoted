//go:build windows

package recorder

import (
	"context"
	"fmt"
	"log/slog"
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
	opMu   sync.Mutex

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
	r.opMu.Lock()
	defer r.opMu.Unlock()

	started := time.Now()
	defer func() {
		slog.Info("recorder start finished",
			"backend", r.Name(),
			"duration_ms", time.Since(started).Milliseconds(),
			"session_dir", r.status.SessionDir,
		)
	}()

	r.mu.Lock()
	if r.status.Status == StatusRecording {
		r.mu.Unlock()
		return fmt.Errorf("already recording")
	}
	r.mu.Unlock()

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
		rate = wasapi.CanonicalSampleRate
	}
	channels := sess.Channels
	if channels <= 0 {
		channels = wasapi.CanonicalChannels
	}

	dual, err := wasapi.StartDualRecorder(wasapi.DualRecorderConfig{
		LoopbackID: loopID,
		CaptureID:  capID,
		SampleRate: uint32(rate),
		Channels:   uint32(channels),
		OnPCM: func(pcm []byte) {
			r.mu.Lock()
			w := r.writer
			r.mu.Unlock()
			if w != nil {
				w.WritePCM(pcm)
			}
		},
		OnLoopPCM: sess.OnSystemPCM,
		OnMicPCM:  sess.OnMicPCM,
	})
	if err != nil {
		slog.Error("wasapi capture start failed", "err", err)
		return fmt.Errorf("start wasapi capture: %w", err)
	}

	diag := dual.Diagnostics()
	slog.Info("wasapi capture started",
		"configured_rate", diag.ConfiguredRate,
		"configured_channels", diag.ConfiguredChannels,
		"loop_internal_rate", diag.LoopInternalRate,
		"loop_internal_channels", diag.LoopInternalChannels,
		"mic_internal_rate", diag.MicInternalRate,
		"mic_internal_channels", diag.MicInternalChannels,
	)

	r.mu.Lock()
	defer r.mu.Unlock()
	r.writer = NewWAVWriter(rate, channels)
	r.dual = dual

	recordStarted := time.Now()
	if err := session.WriteMetadataFile(dir, session.Metadata{
		StartedAt:          recordStarted,
		Provider:           sess.Provider,
		Platform:           sess.Platform,
		Backend:            r.Name(),
		SystemDevice:       devs.system,
		MicDevice:          devs.mic,
		OutputSampleRate:   rate,
		Channels:           channels,
		SystemInternalRate: int(diag.LoopInternalRate),
		MicInternalRate:    int(diag.MicInternalRate),
		AutoRecord:         sess.AutoRecord,
		Manual:             sess.Manual,
	}); err != nil {
		r.dual.Stop()
		r.dual = nil
		r.writer = nil
		return err
	}

	r.status = RecorderStatus{
		Status:     StatusRecording,
		Backend:    r.Name(),
		SessionDir: dir,
		StartedAt:  recordStarted,
	}
	return nil
}

func (r *WindowsWASAPIRecorder) Stop(_ context.Context) error {
	r.opMu.Lock()
	defer r.opMu.Unlock()

	stopBegin := time.Now()

	r.mu.Lock()
	if r.status.Status != StatusRecording {
		r.mu.Unlock()
		return nil
	}
	dual := r.dual
	writer := r.writer
	sessionDir := r.status.SessionDir
	startedAt := r.status.StartedAt
	r.dual = nil
	r.writer = nil
	r.status.Status = StatusStopping
	r.mu.Unlock()

	if dual != nil {
		dual.Stop()
	}

	var writeErr error
	if writer != nil && sessionDir != "" {
		path := dirFile(sessionDir, SessionAudioFile)
		if err := os.WriteFile(path, writer.Bytes(), 0o644); err != nil {
			writeErr = fmt.Errorf("write %s: %w", SessionAudioFile, err)
		}
	}

	ended := time.Now()
	var metaErr error
	if writeErr == nil && sessionDir != "" {
		metaErr = session.UpdateMetadataEnded(sessionDir, startedAt, ended)
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if writeErr != nil {
		r.status.Status = StatusError
		r.status.Error = writeErr.Error()
		slog.Error("recorder stop failed", "phase", "write_wav", "err", writeErr,
			"duration_ms", time.Since(stopBegin).Milliseconds())
		return writeErr
	}
	if metaErr != nil {
		r.status.Status = StatusError
		r.status.Error = metaErr.Error()
		slog.Error("recorder stop failed", "phase", "write_metadata", "err", metaErr,
			"duration_ms", time.Since(stopBegin).Milliseconds())
		return metaErr
	}

	r.status.Status = StatusIdle
	r.status.StartedAt = time.Time{}
	slog.Info("recorder stop finished",
		"backend", r.Name(),
		"session_dir", sessionDir,
		"duration_ms", time.Since(stopBegin).Milliseconds(),
	)
	return nil
}

func (r *WindowsWASAPIRecorder) Status() RecorderStatus {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.status
}
