package recorder

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"anoted/internal/session"
)

// DummyRecorder creates empty WAV placeholders and metadata for MVP testing.
//
// The mutex is not optional: Start runs on its own goroutine (see the TUI's
// record command) while the poll tick reads Status from the Bubble Tea loop, and
// this recorder is reachable in production as the fallback backend.
type DummyRecorder struct {
	mu      sync.Mutex
	backend string
	status  RecorderStatus
	// unusable is empty when the dummy was asked for on purpose
	// (--dummy-recorder) and carries the reason when it is standing in for a
	// real backend that could not be built.
	unusable string
}

// NewDummyRecorder returns the placeholder backend, explicitly requested via
// --dummy-recorder. It is usable in the sense that the caller asked for it.
func NewDummyRecorder() *DummyRecorder {
	return &DummyRecorder{
		backend: "dummy",
		status:  RecorderStatus{Status: StatusIdle, Backend: "dummy"},
	}
}

// NewUnavailableRecorder returns a recorder that refuses to record and explains
// why. Used when no real backend could be built, so the failure is visible up
// front instead of after an hour of empty recording.
func NewUnavailableRecorder(reason string) *DummyRecorder {
	r := NewDummyRecorder()
	r.unusable = reason
	return r
}

func (r *DummyRecorder) Unusable() string { return r.unusable }

func (r *DummyRecorder) Name() string { return "dummy" }

func (r *DummyRecorder) Start(_ context.Context, cfg SessionConfig) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.unusable != "" {
		return fmt.Errorf("cannot record: %s", r.unusable)
	}
	if r.status.Status == StatusRecording {
		return fmt.Errorf("already recording")
	}

	dir, err := createSessionDir(cfg)
	if err != nil {
		return err
	}

	if err := os.WriteFile(filepath.Join(dir, SessionAudioFile), minimalWAV(cfg.SampleRate, cfg.Channels), SessionFileMode); err != nil {
		return fmt.Errorf("write %s: %w", SessionAudioFile, err)
	}

	started := time.Now()
	meta := session.Metadata{
		StartedAt:  started,
		Provider:   cfg.Provider,
		Platform:   cfg.Platform,
		Backend:    r.backend,
		AutoRecord: cfg.AutoRecord,
		Manual:     cfg.Manual,
	}
	if err := session.WriteMetadataFile(dir, meta); err != nil {
		return fmt.Errorf("write metadata: %w", err)
	}

	r.status = RecorderStatus{
		Status:     StatusRecording,
		Backend:    r.backend,
		SessionDir: dir,
		StartedAt:  started,
	}
	return nil
}

func (r *DummyRecorder) Stop(_ context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.status.Status != StatusRecording {
		return nil
	}

	ended := time.Now()
	meta := session.Metadata{
		StartedAt: r.status.StartedAt,
		EndedAt:   ended,
		Duration:  ended.Sub(r.status.StartedAt).Round(time.Second).String(),
		Backend:   r.backend,
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

func (r *DummyRecorder) Status() RecorderStatus {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.status
}

// minimalWAV returns a valid empty mono/stereo WAV header with no samples.
func minimalWAV(sampleRate, channels int) []byte {
	if sampleRate <= 0 {
		sampleRate = 48000
	}
	if channels <= 0 {
		channels = 2
	}
	bitsPerSample := 16
	byteRate := sampleRate * channels * bitsPerSample / 8
	blockAlign := channels * bitsPerSample / 8
	dataSize := 0
	fileSize := 36 + dataSize

	buf := make([]byte, 44)
	copy(buf[0:4], "RIFF")
	putLE32(buf[4:8], uint32(fileSize))
	copy(buf[8:12], "WAVE")
	copy(buf[12:16], "fmt ")
	putLE32(buf[16:20], 16)
	putLE16(buf[20:22], 1)
	putLE16(buf[22:24], uint16(channels))
	putLE32(buf[24:28], uint32(sampleRate))
	putLE32(buf[28:32], uint32(byteRate))
	putLE16(buf[32:34], uint16(blockAlign))
	putLE16(buf[34:36], uint16(bitsPerSample))
	copy(buf[36:40], "data")
	putLE32(buf[40:44], uint32(dataSize))
	return buf
}

func putLE16(b []byte, v uint16) {
	b[0] = byte(v)
	b[1] = byte(v >> 8)
}

func putLE32(b []byte, v uint32) {
	b[0] = byte(v)
	b[1] = byte(v >> 8)
	b[2] = byte(v >> 16)
	b[3] = byte(v >> 24)
}
