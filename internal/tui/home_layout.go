package tui

import (
	"anoted/internal/config"
	"anoted/internal/tui/components"
	"charm.land/lipgloss/v2"
)

// homeLayout is how Home splits the terminal's rows between its panels.
//
// The table's page size used to be the height minus a fixed 25 rows of
// "chrome". The real figure is 31 to 47 depending on width and state, so on
// an 80x24, 100x30 or even 120x40 terminal the frame overflowed and the
// details and preview panels were cut off below the fold.
type homeLayout struct {
	compactTop bool
	pageSize   int
	// bottomRows is passed to SessionsView.BottomRows.
	bottomRows int
}

const (
	// homeFixedRows: header, tab bar, the blank row under them, and the
	// footer's rule and hints.
	homeFixedRows = 5
	// homeTableChrome: the table's borders and its column header.
	homeTableChrome = 3
	// homeTableComfort is how many sessions are shown before the details and
	// preview panels get any room.
	homeTableComfort = 6
	// homeBottomMin is the shortest details panel worth drawing: its borders
	// and three lines.
	homeBottomMin = 5
	// homeStackedPreviewRows is the preview's share when it sits under details.
	homeStackedPreviewRows = 6
)

// homeCompactTop folds the audio panel into the status panel when the
// terminal is short. Stacked (narrow) layouts lose the most: status, a blank
// row and the audio panel take fourteen rows there.
func (m Model) homeCompactTop() bool {
	if m.height <= 0 {
		return false
	}
	stacked := components.NewPanelLayout(m.width).Width < components.HomeTopRowMinWidth
	return m.height < 34 || (stacked && m.height < 46)
}

// homeLayoutKey is every input of homeLayout. The page size is asked for a
// dozen times per frame (records, page count, selection, cache keys), and
// measuring the panels each time cost a fifth of a warm frame.
type homeLayoutKey struct {
	width, height    int
	themeGen         uint64
	recording        bool
	appState         AppState
	provider         string
	detectionTitle   string
	sessionDir       string
	awaitingConfirm  bool
	statusNote       string
	detectionWarn    string
	errMsg           string
	audioUsage       string
	systemDevice     string
	micDevice        string
	levelAvailable   bool
	monitorWarn      string
	levelEnabled     bool
	sessions         int
	transcribeActive bool
	desktopNote      string
}

func (m Model) homeLayout() homeLayout {
	if m.height <= 0 {
		return homeLayout{pageSize: sessionsPageSizeDefault}
	}
	key := homeLayoutKey{
		width: m.width, height: m.height, themeGen: components.ThemeGen(),
		recording: m.recording, appState: m.appState, provider: m.provider,
		detectionTitle: m.detection.Title, sessionDir: m.sessionDir,
		awaitingConfirm: m.awaitingRecordConfirm, statusNote: m.statusNote,
		detectionWarn: m.detection.Warning, errMsg: m.errMsg, audioUsage: m.audioUsageText(),
		systemDevice: m.systemDevice, micDevice: m.micDevice, levelAvailable: m.levelAvailable,
		monitorWarn: m.audioMonitorWarn, levelEnabled: config.LevelMeterEnabled(m.deps.Config),
		sessions: len(m.sessions), transcribeActive: m.transcribeActive, desktopNote: m.sessionsDesktopNote,
	}
	if m.cache != nil && m.cache.layoutSet && m.cache.layoutKey == key {
		return m.cache.layout
	}
	l := m.measureHomeLayout()
	if m.cache != nil {
		m.cache.layoutKey, m.cache.layout, m.cache.layoutSet = key, l, true
	}
	return l
}

func (m Model) measureHomeLayout() homeLayout {
	l := homeLayout{compactTop: m.homeCompactTop(), pageSize: sessionsPageSizeDefault}
	rows := m.height - homeFixedRows - m.homeTopRows(l.compactTop) - 1 - homeTableChrome
	if m.audioUsageText() != "" {
		rows-- // the disk-usage line at the top of the table
	}
	if m.sessionsDesktopNote != "" {
		rows--
	}

	want := components.DetailsRows(m.transcribeActive)
	if components.NewPanelLayout(m.width).FullWidth() < components.HomeTopRowMinWidth {
		want += 1 + homeStackedPreviewRows
	}

	table := min(max(rows, 0), homeTableComfort, max(len(m.sessions), 1))
	rest := rows - table
	l.bottomRows = -1
	if rest-2 >= homeBottomMin { // 2: the blank row above the panels
		l.bottomRows = min(want, rest-2)
		rest -= l.bottomRows + 2
	}
	table += max(rest, 0)
	// Past the table's maximum, the spare rows go to the preview so a tall
	// terminal shows more of the transcript instead of blank space.
	if table > sessionsPageSizeMax {
		if l.bottomRows > 0 {
			l.bottomRows += table - sessionsPageSizeMax
		}
		table = sessionsPageSizeMax
	}
	l.pageSize = max(table, sessionsPageSizeMin)
	return l
}

// homeTopRows measures the status and audio panels as they will render.
func (m Model) homeTopRows(compact bool) int {
	v := m.homeViewBase(compact)
	status := lipgloss.Height(m.cachedStatusBox(v))
	if compact {
		return status
	}
	audio := m.cachedAudioRows(v)
	if components.NewPanelLayout(m.width).Width >= components.HomeTopRowMinWidth {
		return max(status, audio)
	}
	return status + 1 + audio
}

// audioRowsKey fingerprints what decides the audio panel's height; the bands
// themselves only change what is drawn in it.
type audioRowsKey struct {
	width          int
	recording      bool
	levelEnabled   bool
	levelAvailable bool
	monitorWarn    string
	noSystem       bool
	noMic          bool
}

func (m Model) cachedAudioRows(v components.HomeView) int {
	key := audioRowsKey{
		width:          m.width,
		recording:      v.Recording,
		levelEnabled:   v.LevelEnabled,
		levelAvailable: v.LevelAvailable,
		monitorWarn:    v.MonitorWarn,
		noSystem:       v.SystemDevice == "",
		noMic:          v.MicDevice == "",
	}
	if m.cache != nil && m.cache.audioRows > 0 && m.cache.audioKey == key {
		return m.cache.audioRows
	}
	rows := v.AudioRows()
	if m.cache != nil {
		m.cache.audioKey = key
		m.cache.audioRows = rows
	}
	return rows
}

// homeViewBase is everything on Home except the sessions block, which
// depends on the page size this file computes from it.
func (m Model) homeViewBase(compact bool) components.HomeView {
	duration := m.recordElapsed
	if !m.recording {
		duration = 0
	}
	return components.HomeView{
		AppState:        string(m.appState),
		SystemDevice:    m.systemDevice,
		MicDevice:       m.micDevice,
		Provider:        displayProvider(m.provider, m.detection.Title),
		Recording:       m.recording,
		Duration:        duration,
		SessionDir:      m.sessionDir,
		AutoRecord:      m.autoRecord,
		AwaitingConfirm: m.awaitingRecordConfirm,
		ConfirmPrompt:   "Meeting detected — start recording? [y/n]",
		StatusNote:      m.statusNote,
		DetectionWarn:   m.detection.Warning,
		ErrorMsg:        m.errMsg,
		AudioUsage:      m.audioUsageText(),
		Width:           m.width,
		Height:          m.height,

		SystemBands:    m.systemBands,
		MicBands:       m.micBands,
		LevelEnabled:   config.LevelMeterEnabled(m.deps.Config),
		LevelAvailable: m.levelAvailable,
		MonitorWarn:    m.audioMonitorWarn,
		LevelFrame:     m.levelFrame,
		CompactTop:     compact,
	}
}

// repage keeps the selected session selected when the page size changes, which
// now happens whenever a panel above the table grows or shrinks, not only on
// resize.
func (m Model) repage() Model {
	size := m.sessionsPageSize()
	if m.pageSizeSeen != 0 && size != m.pageSizeSeen && len(m.sessions) > 0 {
		global := m.sessionsPage*m.pageSizeSeen + m.sessionCursor
		m.sessionsPage = global / size
		m.sessionCursor = global % size
		m.pageSizeSeen = size
		return m.clampSessionsCursor()
	}
	m.pageSizeSeen = size
	return m
}
