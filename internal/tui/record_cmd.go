package tui

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"anoted/internal/recorder"
)

const recordStartTimeout = 10 * time.Second

func startRecorderWithTimeout(rec recorder.Recorder, ctx context.Context, cfg recorder.SessionConfig) error {
	return startRecorderWithTimeoutDur(rec, ctx, cfg, recordStartTimeout)
}

func startRecorderWithTimeoutDur(rec recorder.Recorder, ctx context.Context, cfg recorder.SessionConfig, timeout time.Duration) error {
	ch := make(chan error, 1)
	go func() {
		ch <- rec.Start(ctx, cfg)
	}()
	select {
	case err := <-ch:
		return err
	case <-time.After(timeout):
		// Recorder.Start is not cancellable, so giving up on the wait does not
		// stop the attempt: it can still bring capture up seconds later. Left
		// alone that produces a live recording the UI believes does not exist —
		// no indicator, no way to stop it, and it outlives quit. Tear it down
		// as soon as it lands.
		go func() {
			if err := <-ch; err == nil {
				slog.Warn("record start completed after timeout; stopping abandoned capture")
				if stopErr := rec.Stop(context.Background()); stopErr != nil {
					slog.Error("failed to stop abandoned capture", "err", stopErr)
				}
			}
		}()
		return fmt.Errorf("record start timed out after %s", timeout)
	}
}
