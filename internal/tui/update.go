package tui

import (
	"context"
	"log/slog"
	"os"
	"time"

	"anoted/internal/config"
	"anoted/internal/level"
	"anoted/internal/recorder"
	"anoted/internal/session"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/term"
)

func (m Model) Init() tea.Cmd {
	m.syncTrayState()
	cmds := []tea.Cmd{
		readWindowSizeCmd(),
		m.schedulePoll(),
		m.scheduleDurationTick(),
		resolveDeviceLabelsCmd(m),
		m.loadSessionsCmd(),
		func() tea.Msg { return homeEnterLevelsMsg{} },
		refreshDoctorCapsCmd(m.deps.Config),
	}
	if poll := m.scheduleWindowSizePoll(); poll != nil {
		cmds = append(cmds, poll)
	}
	return tea.Batch(cmds...)
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
		if resized && m.deps.Platform.ClearScreenOnResize() {
			return m, tea.ClearScreen
		}
		return m, nil
	case TrayQuitMsg:
		// The tray user asked to quit and may not be looking at the TUI, so
		// take the safe path directly instead of popping a confirm dialog:
		// performQuit stops an active recording and flushes it before exiting.
		return m.performQuit()
	case pollTickMsg:
		m = m.reconcileRecordingState()
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
	case autoRecordRetryMsg:
		return m.handleAutoRecordRetry()
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
	case configFolderPickMsg:
		return m.handleConfigFolderPick(msg)
	case transcribeEnvelopeMsg:
		return m.handleTranscribeEnvelope(msg)
	case transcribeBlinkMsg:
		return m.handleTranscribeBlink()
	case whisperInstallEnvelopeMsg:
		return m.handleWhisperInstallEnvelope(msg)
	case whisperInstallSavedMsg:
		return m.handleWhisperInstallSaved(msg)
	case gpuInstallEnvelopeMsg:
		return m.handleGPUInstallEnvelope(msg)
	case gpuInstallSavedMsg:
		return m.handleGPUInstallSaved(msg)
	case doctorCapsMsg:
		m = m.handleDoctorCaps(msg)
		return m, nil
	case setupInstallEnvelopeMsg:
		return m.handleSetupInstallEnvelope(msg)
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

	if m.recordOpInFlight && !m.recordOpAt.IsZero() && now.Sub(m.recordOpAt) > 30*time.Second {
		slog.Warn("record op watchdog fired", "op_age_ms", now.Sub(m.recordOpAt).Milliseconds())
		m.recordOpInFlight = false
		m.recordOpAt = time.Time{}
		if m.autoRecord && m.detection.InMeeting && !m.recording {
			m.wantAutoRecordResume = true
			m.resumeForSessionKey = meetingSessionKey(m.detection)
			m.autoRecordRetryAfter = time.Time{}
		}
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

	wasInMeeting := m.detection.InMeeting
	newSession := detectNewMeetingSession(wasInMeeting, m.lastMeetingSessionKey, msg.snap.State)
	if msg.snap.State.InMeeting {
		slog.Debug("detection poll",
			"in_meeting", msg.snap.State.InMeeting,
			"provider", msg.snap.State.Provider,
			"title", msg.snap.State.Title,
			"was_in_meeting", wasInMeeting,
			"new_session", newSession,
			"recording", m.recording,
			"want_resume", m.wantAutoRecordResume,
		)
	}
	if newSession {
		m.lastAutoStopAt = time.Time{}
		m.recordConfirmDismissed = false
		m.awaitingRecordConfirm = false
		m.autoRecordFailures = 0
	}

	m.detection = msg.snap.State
	if msg.snap.State.InMeeting {
		m.lastMeetingSessionKey = meetingSessionKey(msg.snap.State)
	} else {
		m.lastMeetingSessionKey = ""
		m.lastAutoStopAt = time.Time{}
		m.recordOpInFlight = false
		m.recordOpAt = time.Time{}
	}

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
				m.recordOpAt = now
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
			switch m.autoRecordAction(now, newSession) {
			case autoRecordStart:
				if newSession {
					m.statusNote = ""
				}
				m.recordConfirmDismissed = false
				m.awaitingRecordConfirm = false
				return m.dispatchAutoRecordStart(now)
			case autoRecordConfirm:
				if newSession {
					m.statusNote = ""
					m.recordConfirmDismissed = false
				}
				m.awaitingRecordConfirm = true
				m.appState = StateAwaitingRecordConfirm
				return m, nil
			case autoRecordWait, autoRecordNoop:
				m.appState = StateInMeeting
				return m, nil
			}
		}
		m.appState = StateInMeeting
	} else {
		m.appState = StateIdle
		m.provider = "none"
		m.awaitingRecordConfirm = false
		m.recordConfirmDismissed = false
		m.autoRecordFailures = 0
	}
	return m, nil
}

func (m Model) handleAutoRecordRetry() (tea.Model, tea.Cmd) {
	m.autoRecordRetryAfter = time.Time{}
	if !m.autoRecord || m.recording || m.recordOpInFlight || !m.detection.InMeeting {
		return m, nil
	}
	now := time.Now()
	switch m.autoRecordAction(now, false) {
	case autoRecordStart:
		return m.dispatchAutoRecordStart(now)
	case autoRecordConfirm:
		m.awaitingRecordConfirm = true
		m.appState = StateAwaitingRecordConfirm
	}
	return m, nil
}

func (m Model) handleRecordToggle(msg recordToggleResultMsg) (tea.Model, tea.Cmd) {
	m.recordOpInFlight = false
	m.recordOpAt = time.Time{}
	if msg.err != nil {
		m.recording = false
		m.errMsg = msg.err.Error()
		slog.Warn("record toggle failed", "err", msg.err, "in_meeting", m.detection.InMeeting, "auto_record", m.autoRecord)
		if m.autoRecord && m.detection.InMeeting {
			m.autoRecordFailures++
			slog.Warn("auto-record start failed", "failures", m.autoRecordFailures, "err", msg.err)
			if m.autoRecordFailures >= maxAutoRecordFailures {
				m.wantAutoRecordResume = false
				m.resumeForSessionKey = ""
				m.errMsg = autoRecordGiveUpMsg
				m.statusNote = autoRecordGiveUpMsg
				m.appState = StateInMeeting
				return m, nil
			}
			m.wantAutoRecordResume = true
			m.resumeForSessionKey = meetingSessionKey(m.detection)
			m.autoRecordRetryAfter = time.Now().Add(autoRecordRetryDelay)
			m.statusNote = "Record start failed — retrying…"
			m.appState = StateInMeeting
			return m, m.scheduleAutoRecordRetry()
		}
		m.appState = StateError
		return m, nil
	}

	st := m.deps.Recorder.Status()
	m.recStatus = st
	if msg.recording {
		m.recording = true
		m.recordStart = st.StartedAt
		m.sessionDir = st.SessionDir
		m.awaitingRecordConfirm = false
		m.wantAutoRecordResume = false
		m.resumeForSessionKey = ""
		m.recordingSessionKey = meetingSessionKey(m.detection)
		m.autoRecordRetryAfter = time.Time{}
		m.autoRecordFailures = 0
		m.stopWhenMeetingEnds = m.detection.InMeeting
		m.statusNote = ""
		m.appState = StateRecording
		m.systemBands = nil
		m.micBands = nil
		m.syncTrayState()
		return m, m.scheduleDurationTick()
	}

	m.recording = false
	m.stopWhenMeetingEnds = false
	m.meetingAbsentSince = time.Time{}
	if msg.meetingEnded {
		m.lastAutoStopAt = time.Now()
		m.recordConfirmDismissed = false
		m.awaitingRecordConfirm = false
		// Scope the resume to the meeting that just ended: detection has already
		// moved on, so the key comes from the recording rather than m.detection.
		m.wantAutoRecordResume = m.autoRecord && m.recordingSessionKey != ""
		m.resumeForSessionKey = m.recordingSessionKey
	} else {
		m.wantAutoRecordResume = false
		m.resumeForSessionKey = ""
	}
	m.recordingSessionKey = ""
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
	m.syncTrayState()

	var cmds []tea.Cmd
	savedDir := msg.savedDir
	if savedDir == "" {
		savedDir = st.SessionDir
	}
	if savedDir != "" && m.deps.Config.Transcription.AutoAfterRecording {
		var transcribeCmd tea.Cmd
		m, transcribeCmd = m.startTranscribe(savedDir)
		cmds = append(cmds, transcribeCmd)
	}
	cmds = append(cmds, m.restartHomeLevelsAfterRecord())
	// Route the resume through autoRecordAction rather than dispatching
	// directly, so the confirmation requirement and restart cooldown still
	// apply — going straight to dispatch skipped both.
	if m.autoRecord && m.detection.InMeeting && !m.recording {
		now := time.Now()
		switch m.autoRecordAction(now, false) {
		case autoRecordStart:
			model, startCmd := m.dispatchAutoRecordStart(now)
			m = model
			cmds = append(cmds, startCmd)
		case autoRecordConfirm:
			m.awaitingRecordConfirm = true
			m.appState = StateAwaitingRecordConfirm
		}
	}
	return m, tea.Batch(cmds...)
}

// reconcileRecordingState re-derives m.recording from the recorder itself.
//
// m.recording alone drives the recording indicator, the tray icon, the r key
// and the stop-on-quit guard, so any drift between it and the backend produces
// a capture the user can neither see nor stop. Rather than patch each way they
// can diverge, check the authoritative source once per poll.
func (m Model) reconcileRecordingState() Model {
	if m.deps.Recorder == nil || m.recordOpInFlight || m.quitting {
		return m
	}
	st := m.deps.Recorder.Status()
	m.recStatus = st
	actuallyRecording := st.Status == recorder.StatusRecording

	switch {
	case actuallyRecording && !m.recording:
		slog.Warn("recorder is recording but UI was not; reconciling",
			"session_dir", st.SessionDir, "backend", st.Backend)
		m.recording = true
		m.recordStart = st.StartedAt
		m.sessionDir = st.SessionDir
		m.appState = StateRecording
		m.syncTrayState()
	case !actuallyRecording && m.recording:
		slog.Warn("UI showed recording but recorder stopped; reconciling",
			"status", st.Status, "err", st.Error)
		m.recording = false
		m.stopWhenMeetingEnds = false
		if st.Error != "" {
			m.errMsg = st.Error
		}
		if m.detection.InMeeting {
			m.appState = StateInMeeting
		} else {
			m.appState = StateIdle
		}
		m.syncTrayState()
	}
	return m
}

func (m Model) schedulePoll() tea.Cmd {
	return tea.Tick(m.pollInterval(), func(time.Time) tea.Msg { return pollTickMsg{} })
}

func (m Model) scheduleWindowSizePoll() tea.Cmd {
	interval := m.deps.Platform.WindowSizePollInterval()
	if interval <= 0 {
		return nil
	}
	return tea.Tick(interval, func(time.Time) tea.Msg {
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
		for attempt := 0; attempt < 3; attempt++ {
			if attempt > 0 {
				time.Sleep(time.Duration(attempt) * 300 * time.Millisecond)
			}
			attemptStart := time.Now()
			err = startRecorderWithTimeout(rec, context.Background(), sessCfg)
			slog.Info("record start attempt",
				"attempt", attempt+1,
				"duration_ms", time.Since(attemptStart).Milliseconds(),
				"err", err,
				"provider", provider,
				"in_meeting", inMeeting,
			)
			if err == nil {
				break
			}
		}
		if err != nil {
			if mon != nil && config.LevelMeterEnabled(cfg) {
				_ = mon.StartSystem(cfg.Audio.SystemMonitor)
			}
			return recordToggleResultMsg{err: err}
		}
		st := rec.Status()
		if store != nil {
			// A dropped insert here used to be invisible: the audio and
			// metadata.json existed on disk but the session never appeared in
			// the Sessions tab and could not be opened or transcribed.
			if _, err := store.Create(session.Record{
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
			}); err != nil {
				slog.Error("failed to record session in store",
					"session_dir", st.SessionDir, "err", err)
			}
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
		stopBegin := time.Now()
		st := rec.Status()
		sessionDir := st.SessionDir
		err := rec.Stop(context.Background())
		slog.Info("record stop finished",
			"duration_ms", time.Since(stopBegin).Milliseconds(),
			"meeting_ended", becauseMeetingEnded,
			"session_dir", sessionDir,
			"err", err,
		)
		if err != nil {
			return recordToggleResultMsg{err: err}
		}
		if store != nil && sessionDir != "" {
			ended := time.Now()
			recs, listErr := store.List(1)
			switch {
			case listErr != nil:
				slog.Error("failed to load session for update", "session_dir", sessionDir, "err", listErr)
			case len(recs) > 0 && recs[0].Dir == sessionDir:
				recs[0].EndedAt = ended
				recs[0].Status = session.StatusStopped
				recs[0].Metadata.EndedAt = ended
				recs[0].Metadata.Duration = ended.Sub(recs[0].StartedAt).Round(time.Second).String()
				if err := store.Update(recs[0]); err != nil {
					slog.Error("failed to mark session stopped", "session_dir", sessionDir, "err", err)
				}
			default:
				// The row is missing or belongs to a different session, so the
				// recording stays marked active and never shows its duration.
				slog.Warn("no matching session row to close", "session_dir", sessionDir)
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
