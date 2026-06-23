package tui

import (
	"testing"
	"time"

	"anoted/internal/config"
	"anoted/internal/detector"
)

func TestMeetingSessionKey(t *testing.T) {
	key := meetingSessionKey(detector.MeetingState{
		InMeeting: true,
		Provider:  "teams",
		Title:     "Call with Alex | Microsoft Teams",
	})
	if key == "" {
		t.Fatal("expected non-empty key")
	}
}

func TestDetectNewMeetingSession(t *testing.T) {
	state := detector.MeetingState{InMeeting: true, Provider: "teams", Title: "Call A"}
	if !detectNewMeetingSession(false, "", state) {
		t.Fatal("rising edge should be new session")
	}
	key := meetingSessionKey(state)
	if detectNewMeetingSession(true, key, state) {
		t.Fatal("same session should not be new")
	}
	other := detector.MeetingState{InMeeting: true, Provider: "teams", Title: "Call B"}
	if !detectNewMeetingSession(true, key, other) {
		t.Fatal("title change should be new session")
	}
}

func TestShouldBlockAutoStartStaleNow(t *testing.T) {
	stop := time.Now()
	stale := stop.Add(-10 * time.Second)
	if shouldBlockAutoStart(false, stop, stale) {
		t.Fatal("stale CheckedAt must not block auto-start indefinitely")
	}
}

func TestHandleDetectionAutoStartsSecondMeeting(t *testing.T) {
	cfg := config.Default()
	cfg.AutoRecord = true
	cfg.AutoRecordRequiresConfirmation = false

	m := Model{
		deps:       Deps{Config: cfg},
		autoRecord: true,
	}

	first := detector.Snapshot{
		State: detector.MeetingState{
			InMeeting: true,
			Provider:  "teams",
			Title:     "Call one | Microsoft Teams",
		},
		CheckedAt: time.Now(),
	}
	next, cmd := m.handleDetection(detectionResultMsg{snap: first})
	if cmd == nil {
		t.Fatal("first meeting should dispatch start recording")
	}
	nm := next.(Model)
	if !nm.recordOpInFlight {
		t.Fatal("expected record op in flight")
	}

	nm.recording = false
	nm.recordOpInFlight = false
	nm.lastAutoStopAt = time.Now()
	nm.lastMeetingSessionKey = meetingSessionKey(first.State)
	nm.detection = first.State

	second := detector.Snapshot{
		State: detector.MeetingState{
			InMeeting: true,
			Provider:  "teams",
			Title:     "Call two | Microsoft Teams",
		},
		CheckedAt: time.Now(),
	}
	after, cmd := nm.handleDetection(detectionResultMsg{snap: second})
	if cmd == nil {
		t.Fatal("second meeting should auto-start despite recent auto-stop")
	}
	am := after.(Model)
	if !am.recordOpInFlight {
		t.Fatalf("unexpected state: app=%s inFlight=%v", am.appState, am.recordOpInFlight)
	}
}

func TestHandleDetectionResumesAfterAutoStopSameTitle(t *testing.T) {
	cfg := config.Default()
	cfg.AutoRecord = true
	cfg.AutoRecordRequiresConfirmation = true

	meet := detector.MeetingState{
		InMeeting: true,
		Provider:  "google_meet",
		Title:     "Google Meet",
	}
	m := Model{
		deps:                 Deps{Config: cfg},
		autoRecord:           true,
		detection:            meet,
		lastMeetingSessionKey:  meetingSessionKey(meet),
		wantAutoRecordResume:   true,
		recordConfirmDismissed: true,
		statusNote:             "Meeting ended — saved to /tmp/session",
	}

	snap := detector.Snapshot{State: meet, CheckedAt: time.Now()}
	after, cmd := m.handleDetection(detectionResultMsg{snap: snap})
	if cmd == nil {
		t.Fatal("expected resume start after auto-stop")
	}
	am := after.(Model)
	if am.statusNote != "" {
		t.Fatalf("status note should clear: %q", am.statusNote)
	}
	if am.recordConfirmDismissed {
		t.Fatal("resume should clear dismissed confirm")
	}
	if !am.recordOpInFlight {
		t.Fatal("expected start in flight")
	}
}

func TestHandleDetectionClearsDismissedOnNewSession(t *testing.T) {
	cfg := config.Default()
	cfg.AutoRecordRequiresConfirmation = true

	m := Model{
		deps:       Deps{Config: cfg},
		autoRecord: true,
		recordConfirmDismissed: true,
		detection: detector.MeetingState{InMeeting: true, Provider: "teams", Title: "old"},
		lastMeetingSessionKey:  meetingSessionKey(detector.MeetingState{InMeeting: true, Provider: "teams", Title: "old"}),
	}

	snap := detector.Snapshot{
		State: detector.MeetingState{
			InMeeting: true,
			Provider:  "teams",
			Title:     "new call | Microsoft Teams",
		},
		CheckedAt: time.Now(),
	}
	after, _ := m.handleDetection(detectionResultMsg{snap: snap})
	am := after.(Model)
	if am.recordConfirmDismissed {
		t.Fatal("new session should clear dismissed confirm")
	}
	if am.appState != StateAwaitingRecordConfirm {
		t.Fatalf("got app state %q", am.appState)
	}
}
