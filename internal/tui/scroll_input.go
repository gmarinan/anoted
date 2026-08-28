package tui

import (
	"sync"
	"time"

	tea "charm.land/bubbletea/v2"
)

const (
	sessionsScrollMaxPending = 18
	sessionsScrollMaxStep    = 2
	sessionsScrollTick       = 55 * time.Millisecond
)

type sessionScrollTickMsg struct{}

type sessionScrollAccumulator struct {
	mu           sync.Mutex
	pending      int
	tickInFlight bool
}

// The accumulator lives on the Model rather than at package scope. It has to be
// a pointer: SessionScrollFilter runs outside Update and receives a copy of the
// Model, so a value field could not carry state back. Project conventions ask for no
// unnecessary globals, and a process-wide one also meant tests could not run in
// parallel and state leaked between Model instances.
func newSessionScroll() *sessionScrollAccumulator { return &sessionScrollAccumulator{} }

func (a *sessionScrollAccumulator) add(delta int) (scheduleTick bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.pending += delta
	if a.pending > sessionsScrollMaxPending {
		a.pending = sessionsScrollMaxPending
	}
	if a.pending < -sessionsScrollMaxPending {
		a.pending = -sessionsScrollMaxPending
	}
	if !a.tickInFlight {
		a.tickInFlight = true
		return true
	}
	return false
}

func (a *sessionScrollAccumulator) take() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	d := a.pending
	if d > sessionsScrollMaxStep {
		d = sessionsScrollMaxStep
	} else if d < -sessionsScrollMaxStep {
		d = -sessionsScrollMaxStep
	}
	a.pending -= d
	if a.pending == 0 {
		a.tickInFlight = false
	}
	return d
}

func (a *sessionScrollAccumulator) hasPending() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.pending != 0
}

func (a *sessionScrollAccumulator) reset() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.pending = 0
	a.tickInFlight = false
}

func scrollDeltaFromMsg(msg tea.Msg) (delta int, wheelLike bool) {
	switch msg := msg.(type) {
	case tea.MouseWheelMsg:
		switch msg.Button {
		case tea.MouseWheelUp:
			return -1, true
		case tea.MouseWheelDown:
			return 1, true
		}
	case tea.KeyPressMsg:
		switch msg.String() {
		case "pgup":
			return -1, true
		case "pgdown":
			return 1, true
		}
		if msg.Key().IsRepeat {
			switch msg.String() {
			case "up", "k":
				return -1, true
			case "down", "j":
				return 1, true
			}
		}
	}
	return 0, false
}

// SessionScrollFilter coalesces high-frequency wheel and page-scroll input before
// it reaches Update, so infinite-scroll mice cannot flood the event queue.
func SessionScrollFilter(m tea.Model, msg tea.Msg) tea.Msg {
	delta, wheelLike := scrollDeltaFromMsg(msg)
	if !wheelLike {
		return msg
	}
	model, ok := m.(Model)
	if !ok || !model.canSessionScroll() {
		return msg
	}
	if model.scroll == nil {
		return msg
	}
	if model.scroll.add(delta) {
		return sessionScrollTickMsg{}
	}
	return nil
}

func (m Model) canSessionScroll() bool {
	return m.screen == ScreenMain && !m.sessionsDeleteConfirm && !m.sessionsOpenerPicker
}

func (m Model) scheduleSessionScrollTick() tea.Cmd {
	return tea.Tick(sessionsScrollTick, func(time.Time) tea.Msg {
		return sessionScrollTickMsg{}
	})
}

func (m Model) handleSessionScrollTick() (tea.Model, tea.Cmd) {
	if !m.canSessionScroll() {
		m.scroll.resetSafe()
		return m, nil
	}
	delta := m.scroll.take()
	if delta == 0 {
		return m, nil
	}
	m = m.sessionsNavigate(delta)
	if m.scroll.hasPending() {
		return m, m.scheduleSessionScrollTick()
	}
	return m, nil
}

// Nil-safe wrappers: focused unit tests build a Model directly and never call
// NewModel, so the accumulator can legitimately be absent.
func (a *sessionScrollAccumulator) resetSafe() {
	if a != nil {
		a.reset()
	}
}
