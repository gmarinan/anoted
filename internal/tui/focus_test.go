package tui

import (
	"sync"
	"testing"

	"anoted/internal/level"
	"anoted/internal/recorder"
	tea "charm.land/bubbletea/v2"
)

// fakeMonitor counts the level-monitor calls the focus handler makes.
type fakeMonitor struct {
	mu       sync.Mutex
	starts   int
	stops    int
	liveIdle bool
}

func (f *fakeMonitor) Available() bool           { return true }
func (f *fakeMonitor) SetStreamOptions(_, _ int) {}
func (f *fakeMonitor) LiveWhenIdle() bool        { return f.liveIdle }
func (f *fakeMonitor) Read() level.LevelSnapshot { return level.LevelSnapshot{} }
func (f *fakeMonitor) Close() error              { return nil }
func (f *fakeMonitor) StartMic(string) error     { return nil }
func (f *fakeMonitor) StopMic() error            { return nil }

func (f *fakeMonitor) StartSystem(string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.starts++
	return nil
}

func (f *fakeMonitor) StopSystem() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.stops++
	return nil
}

func (f *fakeMonitor) counts() (int, int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.starts, f.stops
}

// drain runs a command so its side effects land, the way the event loop would.
//
// tea.Batch does not execute anything: it returns a BatchMsg carrying the
// commands for the runtime to run. Calling the batch alone therefore looks like
// a no-op, so the batched commands have to be unwrapped and run too.
func drain(cmd tea.Cmd) {
	if cmd == nil {
		return
	}
	switch msg := cmd().(type) {
	case tea.BatchMsg:
		for _, c := range msg {
			drain(c)
		}
	}
}

// Losing focus must never touch the recording. The focus handler exists to save
// CPU on the level meter, which is a display concern; a recording that stopped
// because the user clicked on their browser would lose the meeting.
func TestBlurDoesNotStopARecording(t *testing.T) {
	rec := &fakeRecorder{}
	mon := &fakeMonitor{liveIdle: true}
	m, _ := newTestModel(t, rec)
	m.deps.LevelMonitor = mon
	m.screen = ScreenMain

	m = runCmd(t, m, startRecordingCmd(m))
	if !m.recording {
		t.Fatal("precondition: model should be recording")
	}
	_, stopsBefore := mon.counts()

	model, cmd := m.handleTerminalFocus(false)
	m = model.(Model)
	drain(cmd)

	if !m.recording {
		t.Fatal("blur stopped the recording")
	}
	if st := rec.Status(); st.Status != recorder.StatusRecording {
		t.Fatalf("recorder status = %q after blur, want recording", st.Status)
	}
	// While recording the meter is the only sign audio is arriving, so it is
	// deliberately left running too.
	if _, stopsAfter := mon.counts(); stopsAfter != stopsBefore {
		t.Fatal("blur stopped the level meter during a recording")
	}
}

// Blur while idle is allowed to stop the meter — that is the whole point — but
// it must leave the recorder alone.
func TestBlurStopsOnlyTheMeterWhenIdle(t *testing.T) {
	rec := &fakeRecorder{}
	mon := &fakeMonitor{liveIdle: true}
	m, _ := newTestModel(t, rec)
	m.deps.LevelMonitor = mon
	m.screen = ScreenMain

	model, cmd := m.handleTerminalFocus(false)
	m = model.(Model)
	drain(cmd)

	if _, stops := mon.counts(); stops != 1 {
		t.Fatalf("level monitor stops = %d, want 1", stops)
	}
	if st := rec.Status(); st.Status == recorder.StatusRecording {
		t.Fatal("blur must not touch the recorder")
	}

	// Regaining focus restarts the capture.
	model, cmd = m.handleTerminalFocus(true)
	m = model.(Model)
	drain(cmd)
	if starts, _ := mon.counts(); starts != 1 {
		t.Fatalf("level monitor starts = %d, want 1 after refocus", starts)
	}
}

// Meeting detection and auto-record must keep running unfocused: waiting in the
// background for a meeting is the app's entire purpose.
func TestBlurDoesNotChangeDetection(t *testing.T) {
	rec := &fakeRecorder{}
	m, _ := newTestModel(t, rec)
	m.deps.LevelMonitor = &fakeMonitor{liveIdle: true}

	before := m.pollInterval()
	model, _ := m.handleTerminalFocus(false)
	m = model.(Model)

	if after := m.pollInterval(); after != before {
		t.Fatalf("poll interval changed on blur: %v -> %v", before, after)
	}
	if m.pollDetection() == nil {
		t.Fatal("detection polling stopped while unfocused")
	}
}

// Auto-record has to be able to start a recording while the terminal is not
// focused, which is when meetings actually begin.
func TestRecordingCanStartWhileBlurred(t *testing.T) {
	rec := &fakeRecorder{}
	m, store := newTestModel(t, rec)
	m.deps.LevelMonitor = &fakeMonitor{liveIdle: true}
	m.screen = ScreenMain

	model, _ := m.handleTerminalFocus(false)
	m = model.(Model)

	m = runCmd(t, m, startRecordingCmd(m))
	if !m.recording {
		t.Fatal("a recording must be able to start while unfocused")
	}
	recs, err := store.List(t.Context(), 10)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(recs) != 1 {
		t.Fatalf("expected the session row to be written, got %d rows", len(recs))
	}
}
