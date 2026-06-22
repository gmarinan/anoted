package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"anoted/internal/session"
)

func testSessions(n int) []session.Record {
	out := make([]session.Record, n)
	for i := range out {
		out[i] = session.Record{ID: int64(i + 1)}
	}
	return out
}

func TestSessionsNavigateAcrossPages(t *testing.T) {
	m := Model{sessions: testSessions(8)}
	m.sessionsPage = 0
	m.sessionCursor = 5
	m = m.sessionsNavigate(1)
	if m.sessionsPage != 1 || m.sessionCursor != 0 {
		t.Fatalf("page=%d cursor=%d", m.sessionsPage, m.sessionCursor)
	}
	m = m.sessionsNavigate(-1)
	if m.sessionsPage != 0 || m.sessionCursor != 5 {
		t.Fatalf("back page=%d cursor=%d", m.sessionsPage, m.sessionCursor)
	}
}

func TestSessionsNavigateClampsAtEnds(t *testing.T) {
	m := Model{sessions: testSessions(3)}
	m.sessionsPage = 0
	m.sessionCursor = 0
	m = m.sessionsNavigate(-1)
	if m.sessionCursor != 0 {
		t.Fatalf("cursor=%d want 0", m.sessionCursor)
	}
	m.sessionCursor = 2
	m = m.sessionsNavigate(1)
	if m.sessionCursor != 2 {
		t.Fatalf("cursor=%d want 2", m.sessionCursor)
	}
}

func TestSessionsPageJumpNeverNegativeCursor(t *testing.T) {
	m := Model{sessions: testSessions(2)}
	m.sessionsPage = 1
	m.sessionCursor = 0
	m = m.sessionsPageJump(-1)
	if m.sessionCursor < 0 {
		t.Fatalf("cursor=%d", m.sessionCursor)
	}
	if m.sessionsPage != 0 {
		t.Fatalf("page=%d", m.sessionsPage)
	}
}

func TestSessionScrollAccumulatorCoalescesBurst(t *testing.T) {
	sessionScroll.reset()
	for range 200 {
		sessionScroll.add(1)
	}
	if sessionScroll.pending != sessionsScrollMaxPending {
		t.Fatalf("pending=%d want %d", sessionScroll.pending, sessionsScrollMaxPending)
	}
	total := 0
	for sessionScroll.hasPending() {
		total += sessionScroll.take()
	}
	if total != sessionsScrollMaxPending {
		t.Fatalf("drained=%d want %d", total, sessionsScrollMaxPending)
	}
}

func TestSessionScrollFilterDropsBurst(t *testing.T) {
	sessionScroll.reset()
	m := Model{screen: ScreenMain, sessions: testSessions(10)}
	var ticks int
	for range 50 {
		out := SessionScrollFilter(m, tea.MouseWheelMsg{Button: tea.MouseWheelDown})
		if out != nil {
			ticks++
		}
	}
	if ticks != 1 {
		t.Fatalf("ticks=%d want 1", ticks)
	}
	if sessionScroll.pending != 18 {
		t.Fatalf("pending=%d want 18", sessionScroll.pending)
	}
	sessionScroll.reset()
}
