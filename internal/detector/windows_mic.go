//go:build windows

package detector

import (
	"context"
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

	titles := d.windowTitles(ctx)

	for _, c := range captures {
		if snap, ok := snapshotFromMicCapture(c, d.cfg.Providers); ok {
			return snap, true
		}
		if snap, ok := snapshotFromBrowserMicAndTitles(c, titles, d.cfg.Providers); ok {
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
