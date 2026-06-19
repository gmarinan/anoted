package tui

import (
	"context"
	"time"

	tea "charm.land/bubbletea/v2"
	"meetctl/internal/recorder"
	"meetctl/internal/session"
)

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.schedulePoll(),
		m.scheduleDurationTick(),
		resolveDeviceLabelsCmd(m),
		loadAudioCatalogCmd(m),
	)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		return m.handleKey(msg)
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case pollTickMsg:
		return m, tea.Batch(m.pollDetection(), m.schedulePoll())
	case durationTickMsg:
		if m.recording {
			return m, m.scheduleDurationTick()
		}
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
	case transcribeResultMsg:
		return m.handleTranscribeResult(msg)
	case deviceLabelsMsg:
		m = m.handleDeviceLabels(msg)
		return m, nil
	}
	return m, nil
}

func (m Model) handleDetection(msg detectionResultMsg) (tea.Model, tea.Cmd) {
	m.appState = StateDetecting
	if msg.err != nil {
		m.appState = StateError
		m.errMsg = msg.err.Error()
		return m, nil
	}

	m.detection = msg.snap.State
	if m.recording {
		if m.stopWhenMeetingEnds && !msg.snap.State.InMeeting {
			return m, stopRecordingCmd(m, true)
		}
		m.appState = StateRecording
		return m, nil
	}

	if msg.snap.State.InMeeting {
		m.provider = msg.snap.State.Provider
		if m.autoRecord {
			if !m.deps.Config.AutoRecordRequiresConfirmation {
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
		return m, m.scheduleDurationTick()
	}

	m.recording = false
	m.stopWhenMeetingEnds = false
	if msg.meetingEnded && msg.savedDir != "" {
		m.statusNote = "Meeting ended — saved to " + msg.savedDir
	} else {
		m.sessionDir = ""
	}
	if m.screen == ScreenSessions {
		recs, err := loadSessionRecords(m.deps.Store)
		if err == nil {
			m.sessions = recs
		}
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
		m.transcribing = true
		m.transcribeNote = "auto-transcribing…"
		return m, transcribeSessionCmd(m, savedDir)
	}
	return m, nil
}

func (m Model) schedulePoll() tea.Cmd {
	return tea.Tick(m.pollInterval(), func(time.Time) tea.Msg { return pollTickMsg{} })
}

func (m Model) scheduleDurationTick() tea.Cmd {
	return tea.Tick(m.tickDuration(), func(time.Time) tea.Msg { return durationTickMsg{} })
}

func (m Model) pollDetection() tea.Cmd {
	d := m.deps.Detector
	return func() tea.Msg {
		snap, err := d.Poll(context.Background())
		return detectionResultMsg{snap: snap, err: err}
	}
}

func startRecordingCmd(m Model) tea.Cmd {
	rec := m.deps.Recorder
	store := m.deps.Store
	cfg := m.deps.Config
	provider := m.provider
	autoRecord := m.autoRecord
	inMeeting := m.detection.InMeeting
	platformName := m.deps.Platform.Name()
	sampleRate := m.deps.Config.Audio.SampleRate
	channels := m.deps.Config.Audio.Channels

	return func() tea.Msg {
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
		err = rec.Start(context.Background(), sessCfg)
		if err != nil {
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
