package tui

import (
	"time"

	tea "charm.land/bubbletea/v2"
	"anoted/internal/config"
)

func (m Model) levelMeterEnabled() bool {
	return config.LevelMeterEnabled(m.deps.Config)
}

func (m Model) scheduleLevelTick(gen int) tea.Cmd {
	if m.screen != ScreenMain || !m.levelMeterEnabled() || m.deps.LevelMonitor == nil || !m.deps.LevelMonitor.Available() {
		return nil
	}
	return tea.Tick(m.levelTickInterval(), func(time.Time) tea.Msg {
		return levelTickMsg{gen: gen}
	})
}

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
	m.systemBands = snap.SystemBands
	if m.recording {
		m.micBands = snap.MicBands
	}
	m.levelFrame++
	return m, m.scheduleLevelTick(msg.gen)
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
