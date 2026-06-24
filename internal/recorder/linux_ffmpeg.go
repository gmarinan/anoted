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

// LinuxFFmpegRecorder uses ffmpeg as a fallback backend.
type LinuxFFmpegRecorder struct {
	cfg    config.Config
	status RecorderStatus
	mu     sync.Mutex
	cmd    *exec.Cmd
}

func NewLinuxFFmpegRecorder(cfg config.Config) (*LinuxFFmpegRecorder, bool) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		return nil, false
	}
	return &LinuxFFmpegRecorder{
		cfg:    cfg,
		status: RecorderStatus{Status: StatusIdle, Backend: "ffmpeg"},
	}, true
}

func (r *LinuxFFmpegRecorder) Name() string { return "ffmpeg" }

func (r *LinuxFFmpegRecorder) Start(_ context.Context, sess SessionConfig) error {
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

	cmd, err := startDualFFmpeg(devs, sess, dir)
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

func (r *LinuxFFmpegRecorder) Stop(_ context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	stopCaptureCmd(r.cmd)
	r.cmd = nil
	if r.status.Status == StatusRecording {
		if r.status.SessionDir != "" && !r.status.StartedAt.IsZero() {
			_ = session.UpdateMetadataEnded(r.status.SessionDir, r.status.StartedAt, time.Now())
		}
		r.status.Status = StatusIdle
	}
	return nil
}

func (r *LinuxFFmpegRecorder) Status() RecorderStatus {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.status
}
