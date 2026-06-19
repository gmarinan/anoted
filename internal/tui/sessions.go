package tui

import (
	"fmt"
	"path/filepath"

	tea "charm.land/bubbletea/v2"
	"meetctl/internal/tui/components"
)

func (m Model) handleSessionsKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.sessionCursor > 0 {
			m.sessionCursor--
		}
		return m, nil
	case "down", "j":
		if m.sessionCursor < len(m.sessions)-1 {
			m.sessionCursor++
		}
		return m, nil
	case "o":
		return m, m.openSessionPath(false)
	case "p":
		return m, m.openSessionPath(true)
	}
	return m, nil
}

func (m Model) openSessionPath(play bool) tea.Cmd {
	if len(m.sessions) == 0 || m.sessionCursor < 0 || m.sessionCursor >= len(m.sessions) {
		return nil
	}
	dir := m.sessions[m.sessionCursor].Dir
	target := dir
	if play {
		target = filepath.Join(dir, "recording.wav")
	}
	return func() tea.Msg {
		if err := components.OpenPath(target); err != nil {
			return sessionsActionMsg{err: fmt.Errorf("open %s: %w", target, err)}
		}
		return sessionsActionMsg{}
	}
}

type sessionsActionMsg struct {
	err error
}

func (m Model) handleSessionsAction(msg sessionsActionMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.sessionsErr = msg.err.Error()
	}
	return m, nil
}
