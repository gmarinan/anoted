package recorder

import (
	"context"
	"sync"
	"testing"
)

// The TUI starts a recorder on its own goroutine while the poll tick reads
// Status from the Bubble Tea loop. DummyRecorder had no mutex at all and is
// reachable in production as the fallback backend on Linux without ffmpeg and
// on WSL2, so this pattern was a live data race that `go test -race` never
// exercised because no test drove the two concurrently.
func TestDummyRecorderStartAndStatusAreConcurrencySafe(t *testing.T) {
	rec := NewDummyRecorder()
	cfg := SessionConfig{OutputRoot: t.TempDir(), Provider: "google_meet", SampleRate: 8000, Channels: 1}

	var wg sync.WaitGroup
	stop := make(chan struct{})

	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
				_ = rec.Status()
			}
		}
	}()

	if err := rec.Start(context.Background(), cfg); err != nil {
		close(stop)
		wg.Wait()
		t.Fatalf("Start: %v", err)
	}
	if err := rec.Stop(context.Background()); err != nil {
		close(stop)
		wg.Wait()
		t.Fatalf("Stop: %v", err)
	}

	close(stop)
	wg.Wait()

	if st := rec.Status(); st.Status != StatusIdle {
		t.Fatalf("status after stop = %q, want idle", st.Status)
	}
}
