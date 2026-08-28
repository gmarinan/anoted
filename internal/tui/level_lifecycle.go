package tui

import (
	"time"

	"anoted/internal/config"
	tea "charm.land/bubbletea/v2"
)

func (m Model) levelMeterEnabled() bool {
	return config.LevelMeterEnabled(m.deps.Config)
}

func (m Model) scheduleLevelTick(gen int) tea.Cmd {
	return m.scheduleLevelTickAfter(gen, m.levelTickInterval())
}

func (m Model) scheduleLevelTickAfter(gen int, d time.Duration) tea.Cmd {
	if m.screen != ScreenMain || !m.levelMeterEnabled() || m.deps.LevelMonitor == nil || !m.deps.LevelMonitor.Available() {
		return nil
	}
	// On backends fed only by the recorder (Windows), Read returns nil bands
	// whenever nothing is recording, so ticking would repaint an empty meter
	// 30 times a second forever.
	if !m.recording && !m.deps.LevelMonitor.LiveWhenIdle() {
		return nil
	}
	return tea.Tick(d, func(time.Time) tea.Msg {
		return levelTickMsg{gen: gen}
	})
}

const (
	// After this many consecutive unchanged reads the meter is considered
	// quiet and backs off: nothing is moving, so repainting 30x/s only burns
	// battery. The first changed sample snaps straight back to the fast rate.
	levelQuietTicks = 45
	// Upper bound for the backed-off interval. Kept short enough that the
	// meter still feels live the instant audio returns.
	levelQuietInterval = 500 * time.Millisecond
)

func (m Model) startSystemLevelCmd() tea.Cmd {
	mon := m.deps.LevelMonitor
	cfg := m.deps.Config
	if mon == nil || !mon.Available() || !config.LevelMeterEnabled(cfg) {
		return nil
	}
	return func() tea.Msg {
		mon.SetStreamOptions(cfg.Audio.LevelLatencyMsec, cfg.Audio.LevelProcessTimeMsec)
		err := mon.StartSystem(cfg.Audio.SystemMonitor)
		return levelStartMsg{err: err}
	}
}

func (m Model) stopSystemLevelCmd() tea.Cmd {
	mon := m.deps.LevelMonitor
	if mon == nil {
		return nil
	}
	return func() tea.Msg {
		_ = mon.StopSystem()
		return levelStopMsg{}
	}
}

func (m Model) startMicLevelCmd() tea.Cmd {
	mon := m.deps.LevelMonitor
	cfg := m.deps.Config
	if mon == nil || !mon.Available() || !config.LevelMeterEnabled(cfg) {
		return nil
	}
	return func() tea.Msg {
		mon.SetStreamOptions(cfg.Audio.LevelLatencyMsec, cfg.Audio.LevelProcessTimeMsec)
		err := mon.StartMic(cfg.Audio.Microphone)
		return levelStartMsg{err: err}
	}
}

func (m Model) stopMicLevelCmd() tea.Cmd {
	mon := m.deps.LevelMonitor
	if mon == nil {
		return nil
	}
	return func() tea.Msg {
		_ = mon.StopMic()
		return levelStopMsg{}
	}
}

type levelStartMsg struct {
	err error
}

type levelStopMsg struct{}

func (m Model) handleLevelTick(msg levelTickMsg) (tea.Model, tea.Cmd) {
	if msg.gen != m.levelGen || m.screen != ScreenMain {
		return m, nil
	}
	snap := m.deps.LevelMonitor.Read()
	changed := !bandsEqual(m.systemBands, snap.SystemBands)
	m.systemBands = snap.SystemBands
	if m.recording {
		if !bandsEqual(m.micBands, snap.MicBands) {
			changed = true
		}
		m.micBands = snap.MicBands
	}

	// The view is a pure function of the bands, so an unchanged read renders an
	// identical frame. Bubble Tea skips the terminal write in that case, but the
	// Update+View work still happens — so back off the tick itself instead.
	if changed {
		m.levelQuiet = 0
	} else if m.levelQuiet < levelQuietTicks {
		m.levelQuiet++
	}
	m.levelFrame++

	interval := m.levelTickInterval()
	if m.levelQuiet >= levelQuietTicks && interval < levelQuietInterval {
		interval = levelQuietInterval
	}
	return m, m.scheduleLevelTickAfter(msg.gen, interval)
}

// bandsEqual reports whether two spectrum snapshots would render identically.
func bandsEqual(a, b []float64) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// enterHomeLevels starts a single level-tick generation for the Home screen.
func (m Model) enterHomeLevels() (Model, []tea.Cmd) {
	m.levelGen++
	m.systemBands = nil
	m.micBands = nil
	if !m.levelMeterEnabled() {
		return m, []tea.Cmd{m.stopSystemLevelCmd(), m.stopMicLevelCmd()}
	}
	if m.deps.LevelMonitor == nil || !m.deps.LevelMonitor.Available() {
		return m, nil
	}
	gen := m.levelGen
	return m, []tea.Cmd{m.startSystemLevelCmd(), m.scheduleLevelTick(gen)}
}

// leaveHomeLevels invalidates in-flight ticks and stops level streams.
func (m Model) leaveHomeLevels() (Model, []tea.Cmd) {
	m.levelGen++
	m.systemBands = nil
	m.micBands = nil
	return m, []tea.Cmd{m.stopSystemLevelCmd(), m.stopMicLevelCmd()}
}
