package tui

import (
	"time"

	"anoted/internal/session"
	"anoted/internal/tui/components"
)

// viewCache memoizes the expensive, rarely-changing pieces of a frame. While
// the level meter animates, View runs up to 30 times a second, and profiling
// showed ~half of every frame spent re-rendering a sessions block whose inputs
// had not changed since the previous tick.
//
// It hangs off the Model behind a pointer (the scroll accumulator precedent):
// View has a value receiver, so writes to Model fields inside View are
// discarded, but writes through a pointer survive. Rendering stays a pure
// function of the Model — identical models produce identical frames, the cache
// only skips recomputing them — so view_purity_test still holds.
type viewCache struct {
	sessionsKey   sessionsBlockKey
	sessionsBlock string

	footerKey footerKey
	footer    string

	statusKey statusBoxKey
	status    string
}

// sessionsBlockKey fingerprints every input of SessionsView.renderMainContent.
// Slices and maps are captured as identity+length plus the model generation
// counters that bump whenever their contents are replaced.
type sessionsBlockKey struct {
	themeGen uint64
	viewGen  uint64

	recordsPtr *session.Record
	recordsLen int

	cursor    int
	page      int
	pageCount int
	total     int

	errMsg      string
	desktopNote string
	width       int
	height      int

	openerPicker   bool
	openerCursor   int
	currentOpener  string
	openerDetected string

	deleteConfirm bool
	deleteCursor  int

	txActive  bool
	txDir     string
	txPercent float64
	txETA     time.Duration
	txBlink   bool
	txLogPtr  *string
	txLogLen  int
	txErr     string

	previewText string
}

func (m Model) sessionsBlockKey() sessionsBlockKey {
	recs := m.sessionsPageRecords()
	var recPtr *session.Record
	if len(recs) > 0 {
		recPtr = &recs[0]
	}
	var logPtr *string
	if len(m.transcribeLog) > 0 {
		logPtr = &m.transcribeLog[0]
	}
	return sessionsBlockKey{
		themeGen:       components.ThemeGen(),
		viewGen:        m.viewGen,
		recordsPtr:     recPtr,
		recordsLen:     len(recs),
		cursor:         m.sessionCursor,
		page:           m.sessionsPage,
		pageCount:      m.sessionsPageCount(),
		total:          len(m.sessions),
		errMsg:         m.sessionsErr,
		desktopNote:    m.sessionsDesktopNote,
		width:          m.width,
		height:         m.height,
		openerPicker:   m.sessionsOpenerPicker,
		openerCursor:   m.sessionsOpenerCursor,
		currentOpener:  m.openerCurrent,
		openerDetected: m.openerDetected,
		deleteConfirm:  m.sessionsDeleteConfirm,
		deleteCursor:   m.sessionsDeleteCursor,
		txActive:       m.transcribeActive,
		txDir:          m.transcribeSessionDir,
		txPercent:      m.transcribePercent,
		txETA:          m.transcribeETA,
		txBlink:        m.transcribeBlink,
		txLogPtr:       logPtr,
		txLogLen:       len(m.transcribeLog),
		txErr:          m.transcribeErr,
		previewText:    m.previewText,
	}
}

// cachedSessionsBlock returns the rendered sessions block, reusing the cached
// string when no render input changed since the last frame.
func (m Model) cachedSessionsBlock() string {
	key := m.sessionsBlockKey()
	if m.cache != nil && m.cache.sessionsBlock != "" && m.cache.sessionsKey == key {
		return m.cache.sessionsBlock
	}
	sess := m.sessionsPanel()
	sess.Height = m.height
	sess.Width = components.NewPanelLayout(m.width).FullWidth()
	block := sess.RenderMainContent()
	if m.cache != nil {
		m.cache.sessionsKey = key
		m.cache.sessionsBlock = block
	}
	return block
}

// statusBoxKey fingerprints every input of the Home status panel. Duration is
// keyed at second granularity — exactly what the panel displays.
type statusBoxKey struct {
	themeGen        uint64
	recording       bool
	appState        AppState
	provider        string
	detectionTitle  string
	autoRecord      bool
	durationSecs    int64
	sessionDir      string
	awaitingConfirm bool
	statusNote      string
	detectionWarn   string
	errMsg          string
	width           int
}

// cachedStatusBox returns the rendered status panel, reused across meter ticks
// until one of its inputs (typically the 1Hz duration) changes.
func (m Model) cachedStatusBox(v components.HomeView) string {
	key := statusBoxKey{
		themeGen:       components.ThemeGen(),
		recording:      m.recording,
		appState:       m.appState,
		provider:       m.provider,
		detectionTitle: m.detection.Title,
		autoRecord:     m.autoRecord,
		// Rounded, not floored: the panel displays Duration.Round(time.Second),
		// so the key must change exactly when the displayed value does.
		durationSecs:    int64(v.Duration.Round(time.Second) / time.Second),
		sessionDir:      m.sessionDir,
		awaitingConfirm: m.awaitingRecordConfirm,
		statusNote:      m.statusNote,
		detectionWarn:   m.detection.Warning,
		errMsg:          m.errMsg,
		width:           m.width,
	}
	if m.cache != nil && m.cache.status != "" && m.cache.statusKey == key {
		return m.cache.status
	}
	box := v.StatusBox(components.StatusPanelWidth(m.width))
	if m.cache != nil {
		m.cache.statusKey = key
		m.cache.status = box
	}
	return box
}

// footerKey fingerprints every input of the footer bar.
type footerKey struct {
	themeGen        uint64
	tab             components.TabID
	quitConfirm     bool
	awaitingConfirm bool
	sessionsMode    components.SessionsFooterMode
	doctorMode      components.DoctorFooterMode
	configMode      components.ConfigFooterMode
	configSaved     string
	configErr       string
	width           int
}

// cachedFooter returns the rendered footer bar; it is a pure function of a
// handful of scalars but costs ~25 style renders and a bordered bar per frame.
func (m Model) cachedFooter(tab components.TabID) string {
	key := footerKey{
		themeGen:        components.ThemeGen(),
		tab:             tab,
		quitConfirm:     m.quitConfirmOpen,
		awaitingConfirm: m.awaitingRecordConfirm,
		sessionsMode:    m.sessionsFooter(),
		doctorMode:      m.doctorFooter(),
		configMode:      m.configFooter(),
		configSaved:     m.configSavedMsg,
		configErr:       m.configErr,
		width:           m.width,
	}
	if m.cache != nil && m.cache.footer != "" && m.cache.footerKey == key {
		return m.cache.footer
	}
	bar := components.FooterBar(m.appFooter(tab), m.width)
	if m.cache != nil {
		m.cache.footerKey = key
		m.cache.footer = bar
	}
	return bar
}
