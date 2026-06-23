package tui

import (
	"context"
	"fmt"
	"time"

	"anoted/internal/recorder"
)

const recordStartTimeout = 10 * time.Second

func startRecorderWithTimeout(rec recorder.Recorder, ctx context.Context, cfg recorder.SessionConfig) error {
	return startRecorderWithTimeoutDur(rec, ctx, cfg, recordStartTimeout)
}

func startRecorderWithTimeoutDur(rec recorder.Recorder, ctx context.Context, cfg recorder.SessionConfig, timeout time.Duration) error {
	type result struct {
		err error
	}
	ch := make(chan result, 1)
	go func() {
		ch <- result{err: rec.Start(ctx, cfg)}
	}()
	select {
	case res := <-ch:
		return res.err
	case <-time.After(timeout):
		return fmt.Errorf("record start timed out after %s", timeout)
	}
}
