package tui

import (
	"fmt"
	"path/filepath"

	"anoted/internal/config"
	"anoted/internal/open"
	"anoted/internal/session"
	"anoted/internal/tui/components"
	tea "charm.land/bubbletea/v2"
)

func (m Model) sessionsPageCount() int {
	if len(m.sessions) == 0 {
		return 1
	}
	return (len(m.sessions) + m.sessionsPageSize() - 1) / m.sessionsPageSize()
}

func (m Model) sessionsPageRecords() []session.Record {
	start := m.sessionsPage * m.sessionsPageSize()
	if start >= len(m.sessions) {
		return nil
	}
	end := start + m.sessionsPageSize()
	if end > len(m.sessions) {
		end = len(m.sessions)
	}
	return m.sessions[start:end]
}

func (m Model) selectedSession() (session.Record, bool) {
	idx := m.sessionsPage*m.sessionsPageSize() + m.sessionCursor
	if idx < 0 || idx >= len(m.sessions) {
		return session.Record{}, false
	}
	return m.sessions[idx], true
}

func (m Model) clampSessionsCursor() Model {
	if len(m.sessions) == 0 {
		m.sessionsPage = 0
		m.sessionCursor = 0
		return m
	}
	global := m.sessionsPage*m.sessionsPageSize() + m.sessionCursor
	if global >= len(m.sessions) {
		global = len(m.sessions) - 1
	}
	if global < 0 {
		global = 0
	}
	m.sessionsPage = global / m.sessionsPageSize()
	m.sessionCursor = global % m.sessionsPageSize()
	pageLen := len(m.sessionsPageRecords())
	if pageLen == 0 {
		m.sessionCursor = 0
	} else if m.sessionCursor >= pageLen {
		m.sessionCursor = pageLen - 1
	}
	if m.sessionCursor < 0 {
		m.sessionCursor = 0
	}
	// Single choke point for selection changes, so the preview is resolved here
	// instead of being read from disk on every frame in View.
	return m.refreshPreview()
}

// sessionsNavigate moves the selection by delta rows across page boundaries.
func (m Model) sessionsNavigate(delta int) Model {
	if len(m.sessions) == 0 {
		m.sessionsPage = 0
		m.sessionCursor = 0
		return m
	}
	global := m.sessionsPage*m.sessionsPageSize() + m.sessionCursor + delta
	if global < 0 {
		global = 0
	}
	if global >= len(m.sessions) {
		global = len(m.sessions) - 1
	}
	m.sessionsPage = global / m.sessionsPageSize()
	m.sessionCursor = global % m.sessionsPageSize()
	return m.clampSessionsCursor()
}

func (m Model) sessionsPageJump(delta int) Model {
	if len(m.sessions) == 0 {
		m.sessionsPage = 0
		m.sessionCursor = 0
		return m
	}
	page := m.sessionsPage + delta
	maxPage := m.sessionsPageCount() - 1
	if page < 0 {
		page = 0
	}
	if page > maxPage {
		page = maxPage
	}
	m.sessionsPage = page
	return m.clampSessionsCursor()
}

func (m Model) handleSessionsKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	if m.sessionsDeleteConfirm {
		return m.handleSessionsDeleteKey(key)
	}
	if m.sessionsOpenerPicker {
		return m.handleSessionsOpenerKey(key)
	}

	switch key {
	case "up", "k":
		m = m.sessionsNavigate(-1)
		return m, nil
	case "down", "j":
		m = m.sessionsNavigate(1)
		return m, nil
	case "[":
		m = m.sessionsPageJump(-1)
		return m, nil
	case "]":
		m = m.sessionsPageJump(1)
		return m, nil
	case "f":
		m.scroll.resetSafe()
		m.sessionsOpenerPicker = true
		m.sessionsOpenerCursor = m.openerCursorIndex()
		m.sessionsDesktopNote = ""
		return m, nil
	case "o":
		return m, m.openSessionPath(false)
	case "p":
		return m, m.openSessionPath(true)
	case "t":
		rec, ok := m.selectedSession()
		if m.transcribeActive || !ok {
			return m, nil
		}
		return m.startTranscribe(rec.Dir)
	case "s":
		if m.transcribeActive {
			return m.stopTranscribe()
		}
		return m, nil
	case "d", "delete":
		rec, ok := m.selectedSession()
		if !ok {
			return m, nil
		}
		if m.recording && rec.Dir == m.sessionDir {
			m.sessionsErr = "cannot delete the active recording session"
			return m, nil
		}
		m.scroll.resetSafe()
		m.sessionsDeleteConfirm = true
		m.sessionsDeleteCursor = 0
		m.sessionsErr = ""
		return m, nil
	}
	return m, nil
}

func (m Model) handleSessionsDeleteKey(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "esc":
		m.sessionsDeleteConfirm = false
		return m, nil
	case "up", "k", "down", "j", "left", "right", "tab":
		m.sessionsDeleteCursor = 1 - m.sessionsDeleteCursor
		return m, nil
	case "enter", " ":
		if m.sessionsDeleteCursor == 0 {
			m.sessionsDeleteConfirm = false
			return m, nil
		}
		m.sessionsDeleteConfirm = false
		return m, m.deleteSelectedSessionCmd()
	}
	return m, nil
}

func (m Model) deleteSelectedSessionCmd() tea.Cmd {
	rec, ok := m.selectedSession()
	if !ok {
		return nil
	}
	store := m.deps.Store
	return func() tea.Msg {
		ctx, cancel := storeContext()
		defer cancel()
		if err := session.Remove(ctx, store, rec); err != nil {
			return sessionsDeletedMsg{err: err}
		}
		records, err := loadSessionRecords(store)
		if err != nil {
			return sessionsDeletedMsg{err: err}
		}
		return sessionsDeletedMsg{records: records, note: "session deleted"}
	}
}

func (m Model) handleSessionsOpenerKey(key string) (tea.Model, tea.Cmd) {
	opts := m.deps.Opener.OpenerOptions(m.deps.Config.Desktop)
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
	current := m.deps.Opener.CurrentOpenerID(m.deps.Config.Desktop)
	opts := m.deps.Opener.OpenerOptions(m.deps.Config.Desktop)
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
	rec, ok := m.selectedSession()
	if !ok {
		return nil
	}
	dir := rec.Dir
	target := dir
	cfg := m.deps.Config
	opener := m.deps.Opener
	if play {
		target = filepath.Join(dir, "recording.wav")
	}
	return func() tea.Msg {
		kind := open.KindFolder
		if play {
			kind = open.KindFile
		}
		if err := opener.Open(target, cfg.Desktop, kind); err != nil {
			return sessionsActionMsg{err: fmt.Errorf("open %s: %w", target, err)}
		}
		return sessionsActionMsg{}
	}
}

type sessionsActionMsg struct {
	err error
}

type sessionsDeletedMsg struct {
	records []session.Record
	note    string
	err     error
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

func (m Model) handleSessionsDeleted(msg sessionsDeletedMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.sessionsErr = msg.err.Error()
		return m, nil
	}
	m.sessions = msg.records
	m.sessionsErr = ""
	if msg.note != "" {
		m.sessionsDesktopNote = msg.note
		m.markStatusTransient()
	}
	m = m.clampSessionsCursor()
	return m, nil
}

func (m Model) handleDesktopOpenerSaved(msg desktopOpenerSavedMsg) (tea.Model, tea.Cmd) {
	m.sessionsOpenerPicker = false
	if msg.err != nil {
		m.sessionsErr = msg.err.Error()
		return m, nil
	}
	m = m.applyConfig(msg.cfg)
	m.sessionsErr = ""
	m.sessionsDesktopNote = fmt.Sprintf("folder opener: %s", m.openerLabel(msg.id))
	m.markStatusTransient()
	return m, doctorReportCmd(m.deps.Config)
}

func (m Model) openerLabel(id string) string {
	for _, o := range m.deps.Opener.OpenerOptions(config.DesktopConfig{Opener: id}) {
		if o.ID == id {
			return o.Label
		}
	}
	return id
}

// sessionsOpenerChoicesIfOpen builds the picker list only when the picker is
// actually visible. Building it unconditionally ran a PATH sweep on every
// View() call — that is, after every message on the Bubble Tea goroutine.
func (m Model) sessionsOpenerChoicesIfOpen() []components.FolderOpenerChoice {
	if !m.sessionsOpenerPicker {
		return nil
	}
	return m.sessionsOpenerChoices()
}

func (m Model) sessionsOpenerChoices() []components.FolderOpenerChoice {
	opts := m.deps.Opener.OpenerOptions(m.deps.Config.Desktop)
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
