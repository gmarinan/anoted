//go:build linux

package recorder

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"sync"
	"time"

	"anoted/internal/config"
	"anoted/internal/session"
)

// LinuxPipeWireRecorder captures via PipeWire/PulseAudio using a single dual-input process.
type LinuxPipeWireRecorder struct {
	cfg     config.Config
	status  RecorderStatus
	mu      sync.Mutex
	capture *captureProc
}

func NewLinuxPipeWireRecorder(cfg config.Config) (*LinuxPipeWireRecorder, bool) {
	if _, err := exec.LookPath("pactl"); err != nil {
		return nil, false
	}
	// ffmpeg is what actually mixes the two inputs and writes recording.wav.
	// Accepting parec instead used to select a capture path that still shelled
	// out to ffmpeg on stop, so every recording on such a machine was lost.
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		return nil, false
	}
	return &LinuxPipeWireRecorder{
		cfg:    cfg,
		status: RecorderStatus{Status: StatusIdle, Backend: "pipewire"},
	}, true
}

func (r *LinuxPipeWireRecorder) Name() string { return "pipewire" }

func (r *LinuxPipeWireRecorder) Start(_ context.Context, sess SessionConfig) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.status.Status == StatusRecording {
		return fmt.Errorf("already recording")
	}

	dir := sessionDir(sess)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	devs, err := resolveFromSession(sess)
	if err != nil {
		return err
	}

	capture, err := startDualCapture(devs, sess, dir)
	if err != nil {
		return err
	}
	r.capture = capture

	started := time.Now()
	_ = session.WriteMetadataFile(dir, session.Metadata{
		StartedAt:    started,
		Provider:     sess.Provider,
		Platform:     sess.Platform,
		Backend:      r.Name(),
		SystemDevice: devs.system,
		MicDevice:    devs.mic,
		AutoRecord:   sess.AutoRecord,
		Manual:       sess.Manual,
	})

	r.status = RecorderStatus{
		Status:     StatusRecording,
		Backend:    r.Name(),
		SessionDir: dir,
		StartedAt:  started,
	}
	return nil
}

func (r *LinuxPipeWireRecorder) Stop(_ context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	stopErr := r.capture.Stop()
	r.capture = nil
	if r.status.Status == StatusRecording {
		if r.status.SessionDir != "" && !r.status.StartedAt.IsZero() {
			_ = session.UpdateMetadataEnded(r.status.SessionDir, r.status.StartedAt, time.Now())
		}
		r.status.Status = StatusIdle
	}
	if stopErr != nil {
		r.status.Error = stopErr.Error()
		return fmt.Errorf("stop capture: %w", stopErr)
	}
	return nil
}

func (r *LinuxPipeWireRecorder) Status() RecorderStatus {
	r.mu.Lock()
	defer r.mu.Unlock()
	// Reconcile against the child process: ffmpeg can die mid-meeting and the
	// UI would otherwise keep showing a healthy, growing recording.
	if r.status.Status == StatusRecording {
		if err := r.capture.Err(); err != nil {
			r.status.Status = StatusError
			r.status.Error = err.Error()
		}
	}
	return r.status
}
