package tui

import (
	"context"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"anoted/internal/config"
	"anoted/internal/platform"
	"anoted/internal/recorder"
	"anoted/internal/session"
	tea "charm.land/bubbletea/v2"
)

// fakeRecorder records the calls made to it and reports whatever status the
// test sets, so the record lifecycle can be driven without touching audio.
type fakeRecorder struct {
	mu        sync.Mutex
	status    recorder.RecorderStatus
	unusable  string
	startErr  error
	stopErr   error
	startedAt time.Time
	dir       string
}

func (f *fakeRecorder) Name() string     { return "fake" }
func (f *fakeRecorder) Unusable() string { return f.unusable }

func (f *fakeRecorder) Start(_ context.Context, cfg recorder.SessionConfig) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.startErr != nil {
		return f.startErr
	}
	f.startedAt = time.Now()
	f.dir = filepath.Join(cfg.OutputRoot, "session")
	f.status = recorder.RecorderStatus{
		Status:     recorder.StatusRecording,
		Backend:    "fake",
		SessionDir: f.dir,
		StartedAt:  f.startedAt,
	}
	return nil
}

func (f *fakeRecorder) Stop(context.Context) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.stopErr != nil {
		return f.stopErr
	}
	f.status = recorder.RecorderStatus{Status: recorder.StatusIdle, Backend: "fake"}
	return nil
}

func (f *fakeRecorder) Status() recorder.RecorderStatus {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.status
}

func newTestModel(t *testing.T, rec recorder.Recorder) (Model, session.Store) {
	t.Helper()
	store := session.NewSQLiteStore(filepath.Join(t.TempDir(), "sessions.db"))
	if err := store.Open(); err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	cfg := config.Default()
	cfg.OutputDir = t.TempDir()

	return NewModel(Deps{
		Config:   cfg,
		Platform: platform.Info{},
		Recorder: rec,
		Store:    store,
	}), store
}

// runCmd drains a tea.Cmd into the model, the way the Bubble Tea loop would.
func runCmd(t *testing.T, m Model, cmd tea.Cmd) Model {
	t.Helper()
	if cmd == nil {
		return m
	}
	msg := cmd()
	if msg == nil {
		return m
	}
	next, _ := m.Update(msg)
	return next.(Model)
}

// The whole start → stop → row-closed path had no test, which is where the
// orphaned-session bug lived: the row was created but re-found by listing the
// newest row, so with two instances the wrong session got closed.
func TestRecordingLifecycleClosesItsOwnRow(t *testing.T) {
	rec := &fakeRecorder{}
	m, store := newTestModel(t, rec)

	m = runCmd(t, m, startRecordingCmd(m))
	if !m.recording {
		t.Fatal("model should be recording after a successful start")
	}
	if m.sessionID == 0 {
		t.Fatal("the created row id must be carried on the model")
	}

	recs, err := store.List(context.Background(), 10)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(recs) != 1 || recs[0].Status != session.StatusActive {
		t.Fatalf("expected one active row, got %+v", recs)
	}
	id := m.sessionID

	m = runCmd(t, m, stopRecordingCmd(m, false))
	if m.recording {
		t.Fatal("model should not be recording after stop")
	}

	closed, err := store.Get(context.Background(), id)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if closed.Status != session.StatusStopped {
		t.Fatalf("status = %q, want stopped", closed.Status)
	}
	if closed.EndedAt.IsZero() {
		t.Fatal("ended_at was never written")
	}
}

// A failed Stop used to return before touching the database, leaving the row
// active forever and skipping auto-transcription of audio that was complete.
func TestFailedStopStillClosesTheRow(t *testing.T) {
	rec := &fakeRecorder{}
	m, store := newTestModel(t, rec)

	m = runCmd(t, m, startRecordingCmd(m))
	id := m.sessionID
	if id == 0 {
		t.Fatal("no row was created")
	}

	rec.mu.Lock()
	rec.stopErr = context.DeadlineExceeded
	rec.mu.Unlock()

	m = runCmd(t, m, stopRecordingCmd(m, false))

	closed, err := store.Get(context.Background(), id)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if closed.Status == session.StatusActive {
		t.Fatal("a failed stop must not leave the session active forever")
	}
	if closed.Status != session.StatusError {
		t.Fatalf("status = %q, want error", closed.Status)
	}
}

// A backend that cannot capture must refuse rather than show a recording
// indicator over a file that will be empty.
func TestUnusableRecorderRefusesToStart(t *testing.T) {
	rec := &fakeRecorder{unusable: "no audio backend available"}
	m, store := newTestModel(t, rec)
	m.screen = ScreenMain

	next, _ := m.handleHomeKey(tea.KeyPressMsg{Code: 'r', Text: "r"})
	m = next.(Model)

	if m.recording {
		t.Fatal("an unusable backend must not put the UI into recording state")
	}
	if m.errMsg == "" {
		t.Fatal("the reason must be shown to the user")
	}
	recs, err := store.List(context.Background(), 10)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(recs) != 0 {
		t.Fatalf("no session row should exist, got %+v", recs)
	}
}

// Auto-record must not burn its retry budget on a backend that cannot capture.
func TestUnusableRecorderBlocksAutoRecord(t *testing.T) {
	rec := &fakeRecorder{unusable: "no audio backend available"}
	m, _ := newTestModel(t, rec)
	m.autoRecord = true
	m.detection.InMeeting = true

	if got := m.autoRecordAction(time.Now(), true); got != autoRecordNoop {
		t.Fatalf("autoRecordAction = %v, want noop for an unusable backend", got)
	}
}
