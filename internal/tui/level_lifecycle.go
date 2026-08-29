package tui

import (
	"time"

	"anoted/internal/config"
	"anoted/internal/level"
	"anoted/internal/tui/components"
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
	// Nobody is looking at an unfocused terminal. Recording keeps ticking so
	// the indicator and duration stay live for whoever does bring it forward.
	if m.blurred && !m.recording {
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

// recorderFeedsLevels reports whether the recorder hands PCM back to the level
// monitor, in which case the monitor must not open the devices itself.
func (m Model) recorderFeedsLevels() bool {
	_, ok := m.deps.LevelMonitor.(level.PCMFeedConfig)
	return ok
}

// micLevelDuringRecordingCmd starts the microphone meter for the duration of a
// recording on backends that do not feed PCM back.
//
// The mic meter was unreachable before: startMicLevelCmd had no callers, and
// the Tab binding the README advertised for switching to it was never handled.
func (m Model) micLevelDuringRecordingCmd() tea.Cmd {
	if m.recorderFeedsLevels() {
		return nil
	}
	return m.startMicLevelCmd()
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
	changed := !components.BandsRenderIdentically(m.systemBands, snap.SystemBands)
	m.systemBands = snap.SystemBands
	if m.recording {
		if !components.BandsRenderIdentically(m.micBands, snap.MicBands) {
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
