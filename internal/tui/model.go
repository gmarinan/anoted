package tui

import (
	"context"
	"time"

	"anoted/internal/audio"
	"anoted/internal/config"
	"anoted/internal/detector"
	"anoted/internal/doctor"
	"anoted/internal/level"
	"anoted/internal/platform"
	"anoted/internal/recorder"
	"anoted/internal/session"
	"anoted/internal/transcribe"
	"anoted/internal/tui/components"
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
	ScreenMain   Screen = "main"
	ScreenDoctor Screen = "doctor"
	ScreenConfig Screen = "config"
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
	LevelMonitor level.Monitor
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
	meetingAbsentSince    time.Time
	recordOpInFlight      bool
	recordOpAt            time.Time
	lastAutoStopAt        time.Time
	lastMeetingSessionKey string
	wantAutoRecordResume  bool
	autoRecordRetryAfter  time.Time
	autoRecordFailures    int
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

	// Resolved device labels for display
	systemDevice string
	micDevice    string
	audioMonitorWarn string

	// Config audio device picker
	configDevicePickerOpen bool
	configDeviceSection    components.AudioSection
	configDeviceCursor     int
	configDeviceCatalog    audio.Catalog
	configDeviceLoading    bool
	configDeviceErr        string

	// Live audio level visualization (Home)
	systemBands []float64
	micBands    []float64
	levelGen    int
	levelFrame  uint64 // bumps each level tick so View content changes every frame

	// Config interactive menu
	configSection      int
	configCursor       int
	configListCursor   int
	configEditing      bool
	configInput        string
	configModalOpen    bool
	configModalCursor  int
	configModalOptions []string
	configErr          string
	configSavedMsg     string

	transcribeActive     bool
	transcribeSessionDir string
	transcribePercent    float64
	transcribeETA        time.Duration
	transcribeLog        []string
	transcribeErr        string
	transcribeBlink      bool
	transcribeCancel     context.CancelFunc

	sessionsOpenerPicker bool
	sessionsOpenerCursor int
	sessionsDesktopNote  string
	sessionsPage         int
	sessionsDeleteConfirm bool
	sessionsDeleteCursor  int
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
	}
}

func (m Model) pollInterval() time.Duration {
	ms := m.deps.Config.Detection.PollIntervalMS
	if ms <= 0 {
		ms = 2000
	}
	return time.Duration(ms) * time.Millisecond
}

func (m Model) levelTickInterval() time.Duration {
	ms := m.deps.Config.Audio.LevelUITickMS
	if ms <= 0 {
		ms = 33
	}
	return time.Duration(ms) * time.Millisecond
}

func (m Model) tickDuration() time.Duration {
	return time.Second
}

type pollTickMsg struct{}
type durationTickMsg struct{}
type levelTickMsg struct {
	gen int
}

type homeEnterLevelsMsg struct{}

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

const sessionsListLimit = 500
const sessionsPageSize = 6

func loadSessionRecords(store session.Store) ([]session.Record, error) {
	if store == nil {
		return nil, nil
	}
	return store.List(sessionsListLimit)
}
