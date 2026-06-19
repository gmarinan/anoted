package tui

import (
	"fmt"
	"path/filepath"

	tea "charm.land/bubbletea/v2"
	"meetctl/internal/config"
	"meetctl/internal/open"
	"meetctl/internal/tui/components"
)

func (m Model) handleSessionsKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	if m.sessionsOpenerPicker {
		return m.handleSessionsOpenerKey(key)
	}

	switch key {
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
	case "f":
		m.sessionsOpenerPicker = true
		m.sessionsOpenerCursor = m.openerCursorIndex()
		m.sessionsDesktopNote = ""
		return m, nil
	case "o":
		return m, m.openSessionPath(false)
	case "p":
		return m, m.openSessionPath(true)
	case "t":
		if m.transcribing || len(m.sessions) == 0 || m.sessionCursor < 0 || m.sessionCursor >= len(m.sessions) {
			return m, nil
		}
		return m.startTranscribe(m.sessions[m.sessionCursor].Dir)
	}
	return m, nil
}

func (m Model) handleSessionsOpenerKey(key string) (tea.Model, tea.Cmd) {
	opts := open.OpenerOptions(m.deps.Config.Desktop)
	switch key {
	case "esc":
		m.sessionsOpenerPicker = false
		return m, nil
	case "up", "k":
		if m.sessionsOpenerCursor > 0 {
			m.sessionsOpenerCursor--
		}
		return m, nil
	case "down", "j":
		if m.sessionsOpenerCursor < len(opts)-1 {
			m.sessionsOpenerCursor++
		}
		return m, nil
	case "enter", " ":
		if m.sessionsOpenerCursor < 0 || m.sessionsOpenerCursor >= len(opts) {
			return m, nil
		}
		opt := opts[m.sessionsOpenerCursor]
		if !opt.Available {
			m.sessionsErr = fmt.Sprintf("%q is not installed", opt.Label)
			return m, nil
		}
		return m, m.saveDesktopOpener(opt.ID)
	}
	return m, nil
}

func (m Model) openerCursorIndex() int {
	current := open.CurrentOpenerID(m.deps.Config.Desktop)
	opts := open.OpenerOptions(m.deps.Config.Desktop)
	for i, o := range opts {
		if o.ID == current {
			return i
		}
	}
	return 0
}

func (m Model) saveDesktopOpener(id string) tea.Cmd {
	path := m.deps.ConfigPath
	cfg := m.deps.Config
	return func() tea.Msg {
		cfg.Desktop.Opener = id
		cfg.Desktop.OpenCommand = nil
		if err := config.Save(path, cfg); err != nil {
			return desktopOpenerSavedMsg{err: err}
		}
		return desktopOpenerSavedMsg{cfg: cfg, id: id}
	}
}

func (m Model) openSessionPath(play bool) tea.Cmd {
	if len(m.sessions) == 0 || m.sessionCursor < 0 || m.sessionCursor >= len(m.sessions) {
		return nil
	}
	dir := m.sessions[m.sessionCursor].Dir
	target := dir
	cfg := m.deps.Config
	if play {
		target = filepath.Join(dir, "recording.wav")
	}
	return func() tea.Msg {
		kind := open.KindFolder
		if play {
			kind = open.KindFile
		}
		if err := open.Open(target, cfg.Desktop, kind); err != nil {
			return sessionsActionMsg{err: fmt.Errorf("open %s: %w", target, err)}
		}
		return sessionsActionMsg{}
	}
}

type sessionsActionMsg struct {
	err error
}

type desktopOpenerSavedMsg struct {
	cfg config.Config
	id  string
	err error
}

func (m Model) handleSessionsAction(msg sessionsActionMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.sessionsErr = msg.err.Error()
	}
	return m, nil
}

func (m Model) handleDesktopOpenerSaved(msg desktopOpenerSavedMsg) (tea.Model, tea.Cmd) {
	m.sessionsOpenerPicker = false
	if msg.err != nil {
		m.sessionsErr = msg.err.Error()
		return m, nil
	}
	m.deps.Config = msg.cfg
	m.sessionsErr = ""
	m.sessionsDesktopNote = fmt.Sprintf("folder opener: %s", openerLabel(msg.id))
	m.doctorReport = loadDoctorReport(m.deps.Config)
	return m, nil
}

func openerLabel(id string) string {
	for _, o := range open.OpenerOptions(config.DesktopConfig{Opener: id}) {
		if o.ID == id {
			return o.Label
		}
	}
	return id
}

func (m Model) sessionsOpenerChoices() []components.FolderOpenerChoice {
	opts := open.OpenerOptions(m.deps.Config.Desktop)
	out := make([]components.FolderOpenerChoice, len(opts))
	for i, o := range opts {
		out[i] = components.FolderOpenerChoice{
			ID:          o.ID,
			Label:       o.Label,
			Description: o.Description,
			Available:   o.Available,
		}
	}
	return out
}
