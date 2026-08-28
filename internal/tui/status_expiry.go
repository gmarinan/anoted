package tui

import "time"

// statusNoteTTL is how long a transient confirmation stays on screen.
//
// None of these notes ever expired or could be dismissed. "✓ saved" sat in the
// Config footer for the rest of the session, so it stopped meaning "that just
// worked" — the only thing it was there to say — and a stale "✓ session
// deleted" outlived the action it described.
const statusNoteTTL = 4 * time.Second

// markStatusTransient starts the countdown for whichever note was just set.
//
// Deliberately not a timer. The detection poll guarantees Update runs
// regularly, so expireStatusNotes riding along on the next message costs no
// extra wakeups — which is the point, given how much of this app's idle cost
// was timers nobody needed.
func (m *Model) markStatusTransient() {
	m.statusExpiry = time.Now().Add(statusNoteTTL)
}

// expireStatusNotes clears transient notes once their deadline has passed.
// Errors are deliberately not cleared here: they stay until the user acts or
// presses esc.
func (m Model) expireStatusNotes(now time.Time) Model {
	if m.statusExpiry.IsZero() || now.Before(m.statusExpiry) {
		return m
	}
	m.statusNote = ""
	m.configSavedMsg = ""
	m.sessionsDesktopNote = ""
	m.statusExpiry = time.Time{}
	return m
}
