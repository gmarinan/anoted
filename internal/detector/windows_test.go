//go:build windows

package detector

import (
	"testing"
	"time"
)

func TestWindowsDetectorModeNone(t *testing.T) {
	d := &windowsDetector{cfg: Config{Mode: ModeNone}}
	snap, err := d.Poll(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if snap.State.InMeeting {
		t.Fatal("expected no meeting")
	}
}

func TestWindowsDetectorMicWarning(t *testing.T) {
	d := &windowsDetector{cfg: Config{Mode: ModeMic, Providers: map[string][]string{
		"google_meet": {"Meet"},
	}}}
	snap, err := d.Poll(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if snap.State.Warning == "" {
		t.Fatal("expected warning for mic mode on Windows")
	}
	_ = snap.CheckedAt.After(time.Time{})
}
