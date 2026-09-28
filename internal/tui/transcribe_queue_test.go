package tui

import (
	"context"
	"errors"
	"strings"
	"testing"

	"anoted/internal/config"
	"anoted/internal/transcribe"
)

func TestEnqueueKeepsMeetingsBehindTheActiveJob(t *testing.T) {
	m := Model{
		transcribeActive:     true,
		transcribeSessionDir: "/meetings/1",
		deps:                 Deps{Transcriber: transcribe.New(config.Config{})},
	}

	m, _ = m.enqueueTranscribe("/meetings/1") // already running
	m, _ = m.enqueueTranscribe("/meetings/2")
	m, _ = m.enqueueTranscribe("/meetings/3")
	m, _ = m.enqueueTranscribe("/meetings/2") // duplicate
	m, _ = m.enqueueTranscribe("  ")

	if !m.transcribeActive || m.transcribeSessionDir != "/meetings/1" {
		t.Fatalf("active job changed: %+v", m.transcribeSessionDir)
	}
	if got := strings.Join(m.transcribeQueue, ","); got != "/meetings/2,/meetings/3" {
		t.Fatalf("queue = %q", got)
	}
	if len(m.transcribeLog) != 2 {
		t.Fatalf("log = %v, want one line per new session", m.transcribeLog)
	}
}

func TestEnqueueStartsWhenIdle(t *testing.T) {
	m := Model{deps: Deps{Transcriber: transcribe.New(config.Config{})}}
	m, cmd := m.enqueueTranscribe("/meetings/1")
	if cmd == nil {
		t.Fatal("expected a transcribe command")
	}
	t.Cleanup(m.cancelTranscribeJob)
	if !m.transcribeActive || m.transcribeSessionDir != "/meetings/1" {
		t.Fatalf("active = %v dir = %q", m.transcribeActive, m.transcribeSessionDir)
	}
	if len(m.transcribeQueue) != 0 {
		t.Fatalf("queue = %v, want empty", m.transcribeQueue)
	}
}

func TestFinishedJobStartsTheNextQueuedSession(t *testing.T) {
	m := Model{
		transcribeActive:     true,
		transcribeSessionDir: "/meetings/1",
		transcribeQueue:      []string{"/meetings/2", "/meetings/3"},
		deps:                 Deps{Transcriber: transcribe.New(config.Config{})},
	}
	model, cmd := m.handleTranscribeResult(transcribeResultMsg{sessionDir: "/meetings/1"})
	m = model.(Model)
	t.Cleanup(m.cancelTranscribeJob)
	if cmd == nil {
		t.Fatal("expected the next job to start")
	}
	if m.transcribeSessionDir != "/meetings/2" || !m.transcribeActive {
		t.Fatalf("now running %q active=%v", m.transcribeSessionDir, m.transcribeActive)
	}
	if got := strings.Join(m.transcribeQueue, ","); got != "/meetings/3" {
		t.Fatalf("queue = %q", got)
	}
	joined := strings.Join(m.transcribeLog, "\n")
	if !strings.Contains(joined, "1 more queued") {
		t.Fatalf("log = %v", m.transcribeLog)
	}
}

func TestFailedJobDoesNotDropTheRestOfTheQueue(t *testing.T) {
	m := Model{
		transcribeActive:     true,
		transcribeSessionDir: "/meetings/1",
		transcribeQueue:      []string{"/meetings/2"},
		deps:                 Deps{Transcriber: transcribe.New(config.Config{})},
	}
	model, _ := m.handleTranscribeResult(transcribeResultMsg{
		sessionDir: "/meetings/1",
		err:        errors.New("whisper: GPU out of memory"),
	})
	m = model.(Model)
	t.Cleanup(m.cancelTranscribeJob)
	if m.transcribeSessionDir != "/meetings/2" {
		t.Fatalf("next = %q", m.transcribeSessionDir)
	}
	if m.sessionsErr != "" {
		t.Fatalf("sessions table replaced by error: %q", m.sessionsErr)
	}
	if m.transcribeErrDir != "/meetings/1" || m.transcribeErr == "" {
		t.Fatalf("failed row not marked: dir=%q err=%q", m.transcribeErrDir, m.transcribeErr)
	}
}

func TestCancelDropsTheQueue(t *testing.T) {
	m := Model{
		transcribeActive:     true,
		transcribeSessionDir: "/meetings/1",
		transcribeQueue:      []string{"/meetings/2", "/meetings/3"},
	}
	model, cmd := m.handleTranscribeResult(transcribeResultMsg{
		sessionDir: "/meetings/1",
		err:        context.Canceled,
	})
	m = model.(Model)
	if cmd != nil {
		t.Fatal("cancel should not start the next session")
	}
	if len(m.transcribeQueue) != 0 || m.transcribeActive {
		t.Fatalf("queue=%v active=%v", m.transcribeQueue, m.transcribeActive)
	}
}

func TestStopClearsQueuedSessions(t *testing.T) {
	m := Model{
		transcribeActive:     true,
		transcribeSessionDir: "/meetings/1",
		transcribeQueue:      []string{"/meetings/2"},
		transcribeCancel:     func() {},
	}
	m, _ = m.stopTranscribe()
	if len(m.transcribeQueue) != 0 {
		t.Fatalf("queue = %v", m.transcribeQueue)
	}
	if !strings.Contains(strings.Join(m.transcribeLog, "\n"), "1 queued cancelled") {
		t.Fatalf("log = %v", m.transcribeLog)
	}
}
