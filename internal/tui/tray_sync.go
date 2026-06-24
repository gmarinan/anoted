package tui

import "anoted/internal/tray"

func (m Model) syncTrayState() {
	if m.deps.Tray == nil {
		return
	}
	if m.recording {
		m.deps.Tray.SetState(tray.StateRecording)
		return
	}
	m.deps.Tray.SetState(tray.StateMonitoring)
}
