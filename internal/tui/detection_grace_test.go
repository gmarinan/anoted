package tui

import (
	"testing"
	"time"

	"anoted/internal/config"
)

func TestShouldStopForMeetingEnd(t *testing.T) {
	grace := 6 * time.Second
	start := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

	stop, absent := shouldStopForMeetingEnd(true, start, start.Add(2*time.Second), grace)
	if stop || !absent.IsZero() {
		t.Fatalf("in meeting should not stop: stop=%v absent=%v", stop, absent)
	}

	stop, absent = shouldStopForMeetingEnd(false, time.Time{}, start, grace)
	if stop || !absent.Equal(start) {
		t.Fatalf("first absent poll should start timer: stop=%v absent=%v", stop, absent)
	}

	stop, absent = shouldStopForMeetingEnd(false, start, start.Add(3*time.Second), grace)
	if stop || !absent.Equal(start) {
		t.Fatalf("within grace should not stop: stop=%v absent=%v", stop, absent)
	}

	stop, absent = shouldStopForMeetingEnd(false, start, start.Add(6*time.Second), grace)
	if !stop || !absent.IsZero() {
		t.Fatalf("after grace should stop: stop=%v absent=%v", stop, absent)
	}
}

func TestShouldBlockAutoStart(t *testing.T) {
	now := time.Now()
	if shouldBlockAutoStart(true, time.Time{}, now) {
		// ok
	} else {
		t.Fatal("in-flight should block")
	}
	if !shouldBlockAutoStart(false, now.Add(-2*time.Second), now) {
		t.Fatal("recent auto-stop should block")
	}
	if shouldBlockAutoStart(false, now.Add(-6*time.Second), now) {
		t.Fatal("old auto-stop should not block")
	}
}

func TestMeetingEndGraceDefault(t *testing.T) {
	if got := meetingEndGrace(config.Config{}); got != 6*time.Second {
		t.Fatalf("got %v", got)
	}
}
