//go:build linux

package recorder

import (
	"context"
	"fmt"
	"os/exec"
	"sync"
	"time"

	"anoted/internal/config"
	"anoted/internal/session"
)

// LinuxFFmpegRecorder uses ffmpeg as a fallback backend.
type LinuxFFmpegRecorder struct {
	cfg     config.Config
	status  RecorderStatus
	mu      sync.Mutex
	capture *captureProc
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

	dir, err := createSessionDir(sess)
	if err != nil {
		return err
	}

	devs, err := resolveFromSession(sess)
	if err != nil {
		return fmt.Errorf("resolve audio devices: %w", err)
	}

	capture, err := startDualFFmpeg(devs, sess, dir)
	if err != nil {
		return fmt.Errorf("start ffmpeg capture: %w", err)
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

func (r *LinuxFFmpegRecorder) Stop(_ context.Context) error {
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

func (r *LinuxFFmpegRecorder) Status() RecorderStatus {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.status.Status == StatusRecording {
		if err := r.capture.Err(); err != nil {
			r.status.Status = StatusError
			r.status.Error = err.Error()
		}
	}
	return r.status
}

// Unusable is always "": constructing this recorder already proved ffmpeg is
// present.
func (r *LinuxFFmpegRecorder) Unusable() string { return "" }
