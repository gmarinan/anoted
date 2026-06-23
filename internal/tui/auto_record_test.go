package tui

import (
	"testing"
	"time"

	"anoted/internal/config"
	"anoted/internal/detector"
)

func TestAutoRecordActionResumeForcesStart(t *testing.T) {
	cfg := config.Default()
	cfg.AutoRecordRequiresConfirmation = true
	m := Model{
		deps:                 Deps{Config: cfg},
		autoRecord:           true,
		wantAutoRecordResume: true,
		recordConfirmDismissed: true,
	}
	if got := m.autoRecordAction(time.Now(), false); got != autoRecordStart {
		t.Fatalf("resume should force start, got %v", got)
	}
}

func TestHandleRecordToggleStartFailureRetries(t *testing.T) {
	m := Model{
		autoRecord: true,
		detection: detector.MeetingState{
			InMeeting: true,
			Provider:  "google_meet",
		},
	}
	after, cmd := m.handleRecordToggle(recordToggleResultMsg{err: errTestRecord})
	am := after.(Model)
	if !am.wantAutoRecordResume {
		t.Fatal("failed start should keep resume intent")
	}
	if am.autoRecordFailures != 1 {
		t.Fatalf("failures=%d want 1", am.autoRecordFailures)
	}
	if cmd == nil {
		t.Fatal("expected retry schedule cmd")
	}
}

func TestHandleRecordToggleStartFailureGivesUp(t *testing.T) {
	m := Model{
		autoRecord: true,
		detection: detector.MeetingState{
			InMeeting: true,
			Provider:  "google_meet",
		},
		autoRecordFailures: maxAutoRecordFailures - 1,
	}
	after, cmd := m.handleRecordToggle(recordToggleResultMsg{err: errTestRecord})
	am := after.(Model)
	if am.wantAutoRecordResume {
		t.Fatal("should stop resume after max failures")
	}
	if am.autoRecordFailures != maxAutoRecordFailures {
		t.Fatalf("failures=%d", am.autoRecordFailures)
	}
	if am.statusNote != autoRecordGiveUpMsg {
		t.Fatalf("status=%q", am.statusNote)
	}
	if cmd != nil {
		t.Fatal("should not schedule retry")
	}
}

// errTestRecord is a sentinel for tests.
var errTestRecord = &testRecordErr{}

type testRecordErr struct{}

func (e *testRecordErr) Error() string { return "test record failure" }
