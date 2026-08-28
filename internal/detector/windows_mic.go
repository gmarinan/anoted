//go:build windows

package detector

import (
	"context"
	"sync"
	"time"

	"anoted/internal/audio"
)

// listMicCapturesHook is overridden in tests to inject session lists.
var listMicCapturesHook = listMicCaptures

func (d *windowsDetector) pollMicOnly(ctx context.Context) (Snapshot, error) {
	snap, found := d.pollMic(ctx)
	if found {
		return snap, nil
	}
	return Snapshot{State: MeetingState{}, CheckedAt: time.Now()}, nil
}

func (d *windowsDetector) pollMic(ctx context.Context) (Snapshot, bool) {
	captures, err := listMicCapturesHook(ctx)
	if err != nil {
		return Snapshot{State: MeetingState{Warning: err.Error()}, CheckedAt: time.Now()}, false
	}

	// windowTitles spawns PowerShell, which costs hundreds of milliseconds of
	// cold-start CPU. When nothing is capturing the mic — the normal idle case —
	// the loop below never runs and the titles are never read, so resolving them
	// eagerly burned a PowerShell process every poll for nothing.
	var (
		titles     []string
		titlesOnce sync.Once
	)
	lazyTitles := func() []string {
		titlesOnce.Do(func() { titles = d.windowTitles(ctx) })
		return titles
	}

	for _, c := range captures {
		if snap, ok := snapshotFromMicCapture(c, d.cfg.Providers); ok {
			return snap, true
		}
		if snap, ok := snapshotFromBrowserMicAndTitles(c, lazyTitles(), d.cfg.Providers); ok {
			return snap, true
		}
	}
	return Snapshot{}, false
}

func listMicCaptures(ctx context.Context) ([]micCapture, error) {
	_ = ctx
	sessions, err := audio.ListCaptureSessions()
	if err != nil {
		return nil, err
	}
	out := make([]micCapture, 0, len(sessions))
	for _, s := range sessions {
		out = append(out, micCapture{
			Binary:    s.ProcessName,
			AppName:   s.DisplayName,
			MediaName: s.DisplayName,
		})
	}
	return out, nil
}
