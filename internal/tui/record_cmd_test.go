package tui

import (
	"context"
	"strings"
	"testing"
	"time"

	"anoted/internal/recorder"
)

type blockingRecorder struct {
	startDelay time.Duration
}

func (b *blockingRecorder) Start(_ context.Context, _ recorder.SessionConfig) error {
	time.Sleep(b.startDelay)
	return nil
}

func (b *blockingRecorder) Stop(context.Context) error      { return nil }
func (b *blockingRecorder) Status() recorder.RecorderStatus { return recorder.RecorderStatus{} }
func (b *blockingRecorder) Name() string                    { return "blocking" }
func (b *blockingRecorder) Unusable() string                { return "" }

func TestStartRecorderWithTimeoutReturnsError(t *testing.T) {
	rec := &blockingRecorder{startDelay: 200 * time.Millisecond}
	err := startRecorderWithTimeoutDur(rec, context.Background(), recorder.SessionConfig{}, 20*time.Millisecond)
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestStartRecorderWithTimeoutSucceeds(t *testing.T) {
	rec := &blockingRecorder{startDelay: 0}
	err := startRecorderWithTimeoutDur(rec, context.Background(), recorder.SessionConfig{}, time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
