package tui

import (
	"context"
	"os"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/term"
	"anoted/internal/config"
	"anoted/internal/level"
	"anoted/internal/recorder"
	"anoted/internal/session"
)

const windowSizePollInterval = 200 * time.Millisecond

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		readWindowSizeCmd(),
		m.scheduleWindowSizePoll(),
		m.schedulePoll(),
		m.scheduleDurationTick(),
		resolveDeviceLabelsCmd(m),
		m.loadSessionsCmd(),
		func() tea.Msg { return homeEnterLevelsMsg{} },
	)
}

type sessionsLoadedMsg struct {
	records []session.Record
	err     error
}

func (m Model) loadSessionsCmd() tea.Cmd {
	store := m.deps.Store
	return func() tea.Msg {
		recs, err := loadSessionRecords(store)
		return sessionsLoadedMsg{records: recs, err: err}
	}
}

type windowSizePollTickMsg struct{}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		return m.handleKey(msg)
	case sessionScrollTickMsg:
		return m.handleSessionScrollTick()
	case windowSizePollTickMsg:
		return m, tea.Batch(readWindowSizeCmd(), m.scheduleWindowSizePoll())
	case tea.WindowSizeMsg:
		resized := m.width != msg.Width || m.height != msg.Height
		if msg.Width > 0 {
			m.width = msg.Width
		}
		if msg.Height > 0 {
			m.height = msg.Height
		}
		if resized {
			return m, tea.ClearScreen
		}
		return m, nil
	case pollTickMsg:
		return m, tea.Batch(m.pollDetection(), m.schedulePoll())
	case durationTickMsg:
		if m.recording {
			return m, m.scheduleDurationTick()
		}
		return m, nil
	case levelTickMsg:
		return m.handleLevelTick(msg)
	case homeEnterLevelsMsg:
		if m.screen == ScreenMain {
			var cmds []tea.Cmd
			m, cmds = m.enterHomeLevels()
			return m, tea.Batch(cmds...)
		}
		return m, nil
	case sessionsLoadedMsg:
		if msg.err != nil {
			m.sessionsErr = msg.err.Error()
			m.sessions = nil
		} else {
			m.sessionsErr = ""
			m.sessions = msg.records
			m = m.clampSessionsCursor()
		}
		return m, nil
	case levelStartMsg:
		if msg.err != nil && m.screen == ScreenMain {
			m.errMsg = msg.err.Error()
		}
		return m, nil
	case levelStopMsg:
		return m, nil
	case detectionResultMsg:
		return m.handleDetection(msg)
	case recordToggleResultMsg:
		return m.handleRecordToggle(msg)
	case audioCatalogMsg:
		return m.handleAudioCatalog(msg)
	case configSavedMsg:
		return m.handleConfigSaved(msg)
	case desktopOpenerSavedMsg:
		return m.handleDesktopOpenerSaved(msg)
	case sessionsActionMsg:
		return m.handleSessionsAction(msg)
	case sessionsDeletedMsg:
		return m.handleSessionsDeleted(msg)
	case configMenuSaveMsg:
		return m.handleConfigMenuSave(msg)
	case transcribeEnvelopeMsg:
		return m.handleTranscribeEnvelope(msg)
	case transcribeBlinkMsg:
		return m.handleTranscribeBlink()
	case deviceLabelsMsg:
		m = m.handleDeviceLabels(msg)
		return m, nil
	}
	return m, nil
}

func (m Model) handleDetection(msg detectionResultMsg) (tea.Model, tea.Cmd) {
	now := msg.snap.CheckedAt
	if now.IsZero() {
		now = time.Now()
	}

	if msg.err != nil {
		if m.recording {
			m.appState = StateRecording
		} else if m.detection.InMeeting {
			m.appState = StateInMeeting
		} else {
			m.appState = StateIdle
		}
		return m, nil
	}

	m.detection = msg.snap.State
	if m.recording {
		if m.stopWhenMeetingEnds {
			stop, absentSince := shouldStopForMeetingEnd(
				msg.snap.State.InMeeting,
				m.meetingAbsentSince,
				now,
				meetingEndGrace(m.deps.Config),
			)
			m.meetingAbsentSince = absentSince
			if stop && !m.recordOpInFlight {
				m.meetingAbsentSince = time.Time{}
				m.recordOpInFlight = true
				return m, stopRecordingCmd(m, true)
			}
		}
		m.appState = StateRecording
		return m, nil
	}

	if msg.snap.State.InMeeting {
		m.meetingAbsentSince = time.Time{}
		m.provider = msg.snap.State.Provider
		if m.autoRecord {
			if !m.deps.Config.AutoRecordRequiresConfirmation {
				if shouldBlockAutoStart(m.recordOpInFlight, m.lastAutoStopAt, now) {
					m.appState = StateInMeeting
					return m, nil
				}
				m.recordOpInFlight = true
				return m, startRecordingCmd(m)
			}
			if m.recordConfirmDismissed {
				m.appState = StateInMeeting
				return m, nil
			}
			m.awaitingRecordConfirm = true
			m.appState = StateAwaitingRecordConfirm
			return m, nil
		}
		m.appState = StateInMeeting
	} else {
		m.appState = StateIdle
		m.provider = "none"
		m.awaitingRecordConfirm = false
		m.recordConfirmDismissed = false
	}
	return m, nil
}

func (m Model) handleRecordToggle(msg recordToggleResultMsg) (tea.Model, tea.Cmd) {
	m.recordOpInFlight = false
	if msg.err != nil {
		m.appState = StateError
		m.errMsg = msg.err.Error()
		m.recording = false
		return m, nil
	}

	st := m.deps.Recorder.Status()
	m.recStatus = st
	if msg.recording {
		m.recording = true
		m.recordStart = st.StartedAt
		m.sessionDir = st.SessionDir
		m.awaitingRecordConfirm = false
		m.stopWhenMeetingEnds = m.detection.InMeeting
		m.statusNote = ""
		m.appState = StateRecording
		m.systemBands = nil
		m.micBands = nil
		return m, m.scheduleDurationTick()
	}

	m.recording = false
	m.stopWhenMeetingEnds = false
	m.meetingAbsentSince = time.Time{}
	if msg.meetingEnded {
		m.lastAutoStopAt = time.Now()
	}
	m.systemBands = nil
	m.micBands = nil
	if msg.meetingEnded && msg.savedDir != "" {
		m.statusNote = "Meeting ended — saved to " + msg.savedDir
	} else {
		m.sessionDir = ""
	}
	if m.screen == ScreenMain {
		m = m.refreshSessions()
	}
	if m.detection.InMeeting {
		m.appState = StateInMeeting
	} else {
		m.appState = StateIdle
	}

	savedDir := msg.savedDir
	if savedDir == "" {
		savedDir = st.SessionDir
	}
	if savedDir != "" && m.deps.Config.Transcription.AutoAfterRecording {
		m, cmd := m.startTranscribe(savedDir)
		return m, tea.Batch(cmd, m.restartHomeLevelsAfterRecord())
	}
	return m, m.restartHomeLevelsAfterRecord()
}

func (m Model) schedulePoll() tea.Cmd {
	return tea.Tick(m.pollInterval(), func(time.Time) tea.Msg { return pollTickMsg{} })
}

func (m Model) scheduleWindowSizePoll() tea.Cmd {
	return tea.Tick(windowSizePollInterval, func(time.Time) tea.Msg {
		return windowSizePollTickMsg{}
	})
}

func (m Model) scheduleDurationTick() tea.Cmd {
	return tea.Tick(m.tickDuration(), func(time.Time) tea.Msg { return durationTickMsg{} })
}

func (m Model) pollDetection() tea.Cmd {
	d := m.deps.Detector
	interval := m.pollInterval()
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), interval/2)
		defer cancel()
		snap, err := d.Poll(ctx)
		return detectionResultMsg{snap: snap, err: err}
	}
}

func startRecordingCmd(m Model) tea.Cmd {
	rec := m.deps.Recorder
	store := m.deps.Store
	cfg := m.deps.Config
	mon := m.deps.LevelMonitor
	provider := m.provider
	autoRecord := m.autoRecord
	inMeeting := m.detection.InMeeting
	platformName := m.deps.Platform.Name()
	sampleRate := m.deps.Config.Audio.SampleRate
	channels := m.deps.Config.Audio.Channels

	return func() tea.Msg {
		if mon != nil {
			_ = mon.StopSystem()
			_ = mon.StopMic()
		}

		outRoot, err := cfg.ResolvedOutputDir()
		if err != nil {
			return recordToggleResultMsg{err: err}
		}
		p := sessionProvider(provider)
		sessCfg := recorder.SessionConfig{
			OutputRoot:    outRoot,
			Provider:      p,
			Platform:      platformName,
			AutoRecord:    autoRecord && inMeeting,
			Manual:        !(autoRecord && inMeeting),
			SampleRate:    sampleRate,
			Channels:      channels,
			SystemMonitor: cfg.Audio.SystemMonitor,
			Microphone:    cfg.Audio.Microphone,
		}
		if feeder, ok := mon.(level.PCMFeedConfig); ok {
			feeder.SetFeedChannels(channels)
			sessCfg.OnSystemPCM = feeder.FeedSystemPCM
			sessCfg.OnMicPCM = feeder.FeedMicPCM
		}
		err = rec.Start(context.Background(), sessCfg)
		if err != nil {
			if mon != nil && config.LevelMeterEnabled(cfg) {
				_ = mon.StartSystem(cfg.Audio.SystemMonitor)
			}
			return recordToggleResultMsg{err: err}
		}
		st := rec.Status()
		if store != nil {
			_, _ = store.Create(session.Record{
				Dir:       st.SessionDir,
				Provider:  p,
				Platform:  platformName,
				Backend:   rec.Name(),
				StartedAt: st.StartedAt,
				Status:    session.StatusActive,
				Metadata: session.Metadata{
					StartedAt:  st.StartedAt,
					Provider:   p,
					Platform:   platformName,
					Backend:    rec.Name(),
					AutoRecord: sessCfg.AutoRecord,
					Manual:     sessCfg.Manual,
				},
			})
		}
		return recordToggleResultMsg{recording: true}
	}
}

func (m Model) restartHomeLevelsAfterRecord() tea.Cmd {
	if m.screen != ScreenMain || !m.levelMeterEnabled() {
		return nil
	}
	if m.deps.LevelMonitor == nil || !m.deps.LevelMonitor.Available() {
		return nil
	}
	return m.startSystemLevelCmd()
}

func stopRecordingCmd(m Model, becauseMeetingEnded bool) tea.Cmd {
	rec := m.deps.Recorder
	store := m.deps.Store
	return func() tea.Msg {
		st := rec.Status()
		sessionDir := st.SessionDir
		err := rec.Stop(context.Background())
		if err != nil {
			return recordToggleResultMsg{err: err}
		}
		if store != nil && sessionDir != "" {
			ended := time.Now()
			recs, listErr := store.List(1)
			if listErr == nil && len(recs) > 0 && recs[0].Dir == sessionDir {
				recs[0].EndedAt = ended
				recs[0].Status = session.StatusStopped
				recs[0].Metadata.EndedAt = ended
				recs[0].Metadata.Duration = ended.Sub(recs[0].StartedAt).Round(time.Second).String()
				_ = store.Update(recs[0])
			}
		}
		return recordToggleResultMsg{
			recording:    false,
			meetingEnded: becauseMeetingEnded,
			savedDir:     sessionDir,
		}
	}
}

func sessionProvider(name string) session.Provider {
	if name == "" || name == "none" {
		return session.ProviderUnknown
	}
	return session.Provider(name)
}

func readWindowSizeCmd() tea.Cmd {
	return func() tea.Msg {
		w, h, err := term.GetSize(os.Stdout.Fd())
		if err != nil || w <= 0 {
			w = 80
		}
		if h <= 0 {
			h = 24
		}
		return tea.WindowSizeMsg{Width: w, Height: h}
	}
}
