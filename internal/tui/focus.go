package tui

import tea "charm.land/bubbletea/v2"

// handleTerminalFocus reacts to the terminal gaining or losing focus.
//
// The level meter exists to be looked at. When the terminal is not focused
// nobody is looking, so there is no reason to sample and repaint it several
// times a second — and anoted's whole job is to sit in the background waiting
// for a meeting, so this is the common case rather than the exception.
//
// This detects focus of the terminal, not whether the window is minimised or on
// another workspace; there is no portable way to ask that, and inventing one is
// not worth it. Terminals without DECSET 1004 support never send these
// messages, so they keep the previous behaviour.
func (m Model) handleTerminalFocus(focused bool) (tea.Model, tea.Cmd) {
	if m.blurred == !focused {
		return m, nil
	}
	m.blurred = !focused

	// Stop the capture itself, not just the repaint. Measured on this machine
	// with the default "responsive" preset, the meter costs ~0.72% of a core:
	// ~0.30% is the parec reader goroutine running peak + FFT + band smoothing
	// on every 20ms chunk, ~0.08% is the parec process, and the rest is
	// repainting. Backing off the UI tick alone leaves the two audio costs
	// untouched, because they are driven by the stream and not by the UI.
	//
	// Keeping the stream open also prevents PipeWire from suspending the sink,
	// which matters more on a laptop than the CPU time does.
	if m.blurred {
		if m.recording {
			// Still recording: the meter is the only feedback that audio is
			// arriving, so leave the existing tick chain running (the
			// scheduler's blurred check explicitly allows this case). Bumping
			// the generation here used to kill the chain with nothing to
			// restart it, freezing the meter for the whole blurred stretch of
			// a recording.
			return m, nil
		}
		// The in-flight tick chain dies with the generation bump.
		m.levelGen++
		return m, m.stopSystemLevelCmd()
	}

	m.levelGen++
	m.levelQuiet = 0
	if m.recording {
		return m, m.scheduleLevelTick(m.levelGen)
	}
	// parec needs ~100ms to produce its first chunk, which is imperceptible
	// against the act of focusing a window.
	return m, tea.Batch(m.startSystemLevelCmd(), m.scheduleLevelTick(m.levelGen))
}
