//go:build windows

package detector

import (
	"context"
	"testing"
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

func TestWindowsDetectorMicFromSessions(t *testing.T) {
	orig := listMicCapturesHook
	defer func() { listMicCapturesHook = orig }()

	listMicCapturesHook = func(context.Context) ([]micCapture, error) {
		return []micCapture{{
			Binary:    "firefox",
			AppName:   "Firefox",
			MediaName: "Meet - Daily standup",
		}}, nil
	}

	d := &windowsDetector{cfg: Config{
		Mode: ModeMic,
		Providers: map[string][]string{
			"google_meet": {"Meet -", "Google Meet"},
		},
	}}
	snap, err := d.Poll(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if !snap.State.InMeeting || snap.State.Provider != ProviderGoogleMeet {
		t.Fatalf("unexpected state: %+v", snap.State)
	}
}

func TestWindowsDetectorMicIdle(t *testing.T) {
	orig := listMicCapturesHook
	defer func() { listMicCapturesHook = orig }()

	listMicCapturesHook = func(context.Context) ([]micCapture, error) {
		return nil, nil
	}

	d := &windowsDetector{cfg: Config{Mode: ModeMic}}
	snap, err := d.Poll(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if snap.State.InMeeting {
		t.Fatal("expected idle when no capture sessions")
	}
}

func TestWindowsDetectorWindowNoProcessOnlyTeams(t *testing.T) {
	orig := listMicCapturesHook
	defer func() { listMicCapturesHook = orig }()
	listMicCapturesHook = func(context.Context) ([]micCapture, error) { return nil, nil }

	d := &windowsDetector{cfg: Config{
		Mode: ModeWindow,
		Providers: map[string][]string{
			"teams": {"Meeting with", "In a call"},
		},
	}}
	snap, err := d.Poll(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if snap.State.InMeeting {
		t.Fatalf("process-only Teams should not mark in meeting: %+v", snap.State)
	}
}
