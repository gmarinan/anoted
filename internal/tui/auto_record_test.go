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
	meet := detector.MeetingState{InMeeting: true, Provider: "google_meet", Title: "standup"}
	m := Model{
		deps:                   Deps{Config: cfg},
		autoRecord:             true,
		detection:              meet,
		wantAutoRecordResume:   true,
		resumeForSessionKey:    meetingSessionKey(meet),
		recordConfirmDismissed: true,
	}
	if got := m.autoRecordAction(time.Now(), false); got != autoRecordStart {
		t.Fatalf("resume should force start for the granting meeting, got %v", got)
	}
}

func TestAutoRecordResumeDoesNotCarryToAnotherMeeting(t *testing.T) {
	// A resume granted by one meeting must not let a later, unrelated meeting
	// skip the confirmation prompt — that silently bypassed
	// auto_record_requires_confirmation for every meeting after the first.
	cfg := config.Default()
	cfg.AutoRecordRequiresConfirmation = true
	granted := detector.MeetingState{InMeeting: true, Provider: "google_meet", Title: "standup"}
	other := detector.MeetingState{InMeeting: true, Provider: "teams", Title: "1:1 with Sam"}
	m := Model{
		deps:                 Deps{Config: cfg},
		autoRecord:           true,
		detection:            other,
		wantAutoRecordResume: true,
		resumeForSessionKey:  meetingSessionKey(granted),
	}
	if got := m.autoRecordAction(time.Now(), true); got != autoRecordConfirm {
		t.Fatalf("a different meeting must still ask for confirmation, got %v", got)
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
