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
	cfg    config.Config
	status RecorderStatus
	mu     sync.Mutex
	cmd    *exec.Cmd
}

func NewLinuxPipeWireRecorder(cfg config.Config) (*LinuxPipeWireRecorder, bool) {
	if _, err := exec.LookPath("pactl"); err != nil {
		return nil, false
	}
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		if _, err := exec.LookPath("parec"); err != nil {
			return nil, false
		}
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

	cmd, err := startDualCapture(devs, sess, dir)
	if err != nil {
		return err
	}
	r.cmd = cmd

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
	stopCaptureCmd(r.cmd)
	r.cmd = nil
	if r.status.Status == StatusRecording {
		r.status.Status = StatusIdle
	}
	return nil
}

func (r *LinuxPipeWireRecorder) Status() RecorderStatus {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.status
}
