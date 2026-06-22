package tui

import (
	"fmt"
	"path/filepath"

	tea "charm.land/bubbletea/v2"
	"anoted/internal/config"
	"anoted/internal/open"
	"anoted/internal/session"
	"anoted/internal/tui/components"
)

func (m Model) sessionsPageCount() int {
	if len(m.sessions) == 0 {
		return 1
	}
	return (len(m.sessions) + sessionsPageSize - 1) / sessionsPageSize
}

func (m Model) sessionsPageRecords() []session.Record {
	start := m.sessionsPage * sessionsPageSize
	if start >= len(m.sessions) {
		return nil
	}
	end := start + sessionsPageSize
	if end > len(m.sessions) {
		end = len(m.sessions)
	}
	return m.sessions[start:end]
}

func (m Model) selectedSession() (session.Record, bool) {
	idx := m.sessionsPage*sessionsPageSize + m.sessionCursor
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
	global := m.sessionsPage*sessionsPageSize + m.sessionCursor
	if global >= len(m.sessions) {
		global = len(m.sessions) - 1
	}
	if global < 0 {
		global = 0
	}
	m.sessionsPage = global / sessionsPageSize
	m.sessionCursor = global % sessionsPageSize
	pageLen := len(m.sessionsPageRecords())
	if pageLen == 0 {
		m.sessionCursor = 0
	} else if m.sessionCursor >= pageLen {
		m.sessionCursor = pageLen - 1
	}
	if m.sessionCursor < 0 {
		m.sessionCursor = 0
	}
	return m
}

// sessionsNavigate moves the selection by delta rows across page boundaries.
func (m Model) sessionsNavigate(delta int) Model {
	if len(m.sessions) == 0 {
		m.sessionsPage = 0
		m.sessionCursor = 0
		return m
	}
	global := m.sessionsPage*sessionsPageSize + m.sessionCursor + delta
	if global < 0 {
		global = 0
	}
	if global >= len(m.sessions) {
		global = len(m.sessions) - 1
	}
	m.sessionsPage = global / sessionsPageSize
	m.sessionCursor = global % sessionsPageSize
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

func (m Model) refreshSessions() Model {
	recs, err := loadSessionRecords(m.deps.Store)
	if err != nil {
		m.sessionsErr = err.Error()
		m.sessions = nil
	} else {
		m.sessionsErr = ""
		m.sessions = recs
	}
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
		sessionScroll.reset()
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
		sessionScroll.reset()
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
		if err := session.Remove(store, rec); err != nil {
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
	rec, ok := m.selectedSession()
	if !ok {
		return nil
	}
	dir := rec.Dir
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
