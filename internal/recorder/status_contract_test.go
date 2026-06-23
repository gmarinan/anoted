package recorder

import (
	"context"
	"sync"
	"testing"
	"time"
)

// slowStopRecorder simulates Stop blocked on teardown while Status must stay responsive.
type slowStopRecorder struct {
	mu     sync.Mutex
	status RecorderStatus
	stopCh chan struct{}
}

func newSlowStopRecorder() *slowStopRecorder {
	return &slowStopRecorder{
		status: RecorderStatus{Status: StatusRecording, Backend: "test"},
		stopCh: make(chan struct{}),
	}
}

func (r *slowStopRecorder) Name() string { return "slow-stop" }

func (r *slowStopRecorder) Start(context.Context, SessionConfig) error { return nil }

func (r *slowStopRecorder) Stop(context.Context) error {
	<-r.stopCh
	r.mu.Lock()
	r.status.Status = StatusIdle
	r.mu.Unlock()
	return nil
}

func (r *slowStopRecorder) Status() RecorderStatus {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.status
}

func (r *slowStopRecorder) releaseStop() { close(r.stopCh) }

func TestStatusDoesNotBlockDuringSlowStop(t *testing.T) {
	rec := newSlowStopRecorder()
	go func() { _ = rec.Stop(context.Background()) }()

	time.Sleep(20 * time.Millisecond)

	start := time.Now()
	st := rec.Status()
	if elapsed := time.Since(start); elapsed > 50*time.Millisecond {
		t.Fatalf("Status blocked for %v while Stop in progress", elapsed)
	}
	if st.Status != StatusRecording {
		t.Fatalf("got status %q", st.Status)
	}

	rec.releaseStop()
}

// TestStopTeardownOutsideStatusLock mirrors the WASAPI pattern: mark stopping under
// lock, then run slow teardown without holding the status lock.
func TestStopTeardownOutsideStatusLock(t *testing.T) {
	var mu sync.Mutex
	status := StatusRecording
	teardownDone := make(chan struct{})

	go func() {
		mu.Lock()
		status = StatusStopping
		mu.Unlock()

		<-teardownDone

		mu.Lock()
		status = StatusIdle
		mu.Unlock()
	}()

	time.Sleep(10 * time.Millisecond)

	start := time.Now()
	mu.Lock()
	_ = status
	mu.Unlock()
	if elapsed := time.Since(start); elapsed > 50*time.Millisecond {
		t.Fatalf("status read blocked for %v", elapsed)
	}

	close(teardownDone)
}
