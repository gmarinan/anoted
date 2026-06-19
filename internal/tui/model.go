package tui

import (
	"time"

	"meetctl/internal/audio"
	"meetctl/internal/config"
	"meetctl/internal/detector"
	"meetctl/internal/doctor"
	"meetctl/internal/platform"
	"meetctl/internal/recorder"
	"meetctl/internal/session"
	"meetctl/internal/transcribe"
	"meetctl/internal/tui/components"
)

// AppState is the high-level UI state.
type AppState string

const (
	StateIdle                  AppState = "idle"
	StateDetecting             AppState = "detecting"
	StateInMeeting             AppState = "in_meeting"
	StateAwaitingRecordConfirm AppState = "awaiting_record_confirm"
	StateRecording             AppState = "recording"
	StateError                 AppState = "error"
)

// Screen is the active TUI screen.
type Screen string

const (
	ScreenMain     Screen = "main"
	ScreenDoctor   Screen = "doctor"
	ScreenSessions Screen = "sessions"
	ScreenConfig   Screen = "config"
)

// Deps bundles injected services for the TUI.
type Deps struct {
	Config     config.Config
	ConfigPath string
	Platform   platform.Info
	Detector   detector.Detector
	Recorder   recorder.Recorder
	Store       session.Store
	Audio       audio.Provider
	Transcriber *transcribe.Service
}

// Model is the Bubble Tea model.
type Model struct {
	deps Deps

	screen       Screen
	appState     AppState
	provider     string
	detection    detector.MeetingState
	recStatus    recorder.RecorderStatus
	autoRecord              bool
	awaitingRecordConfirm   bool
	recordConfirmDismissed  bool
	recording               bool
	stopWhenMeetingEnds     bool
	statusNote              string
	recordStart  time.Time
	sessionDir   string
	errMsg       string
	doctorReport doctor.Report
	sessions     []session.Record
	sessionCursor int
	sessionsErr  string
	width        int
	height       int
	quitting     bool

	// Audio device picker
	audioSection     components.AudioSection
	audioCursor      int
	audioCatalog     audio.Catalog
	audioLoading     bool
	audioErr         string
	audioSaved       string
	audioMonitorWarn string
	systemDevice     string // resolved label for main screen
	micDevice        string

	// Config YAML editor
	configLines     []string
	configCursorRow int
	configCursorCol int
	configScrollRow int
	configDirty     bool
	configErr       string
	configSavedMsg  string

	transcribing   bool
	transcribeNote string
}

// NewModel creates the initial TUI model.
func NewModel(deps Deps) Model {
	return Model{
		deps:       deps,
		screen:     ScreenMain,
		appState:   StateIdle,
		provider:   "none",
		autoRecord:   deps.Config.AutoRecord,
		recStatus:    deps.Recorder.Status(),
		audioLoading: true,
	}
}

func (m Model) pollInterval() time.Duration {
	ms := m.deps.Config.Detection.PollIntervalMS
	if ms <= 0 {
		ms = 2000
	}
	return time.Duration(ms) * time.Millisecond
}

func (m Model) tickDuration() time.Duration {
	return time.Second
}

type pollTickMsg struct{}
type durationTickMsg struct{}

type detectionResultMsg struct {
	snap detector.Snapshot
	err  error
}

type recordToggleResultMsg struct {
	recording    bool
	err          error
	meetingEnded bool
	savedDir     string
}

type audioCatalogMsg struct {
	catalog audio.Catalog
	err     error
}

type deviceLabelsMsg struct {
	system string
	mic    string
	err    error
}

type configSavedMsg struct {
	cfg config.Config
	err error
}

func loadDoctorReport(cfg config.Config) doctor.Report {
	return doctor.Run(cfg)
}

func loadSessionRecords(store session.Store) ([]session.Record, error) {
	if store == nil {
		return nil, nil
	}
	return store.List(50)
}
