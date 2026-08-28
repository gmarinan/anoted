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
	"anoted/internal/setup"
	"anoted/internal/transcribe"
	"anoted/internal/tray"
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
	Config       config.Config
	ConfigPath   string
	Platform     platform.Info
	Detector     detector.Detector
	Recorder     recorder.Recorder
	Store        session.Store
	Audio        audio.Provider
	LevelMonitor level.Monitor
	Transcriber  *transcribe.Service
	Tray         tray.Indicator
}

// Model is the Bubble Tea model.
type Model struct {
	deps Deps

	screen                 Screen
	appState               AppState
	provider               string
	detection              detector.MeetingState
	recStatus              recorder.RecorderStatus
	autoRecord             bool
	awaitingRecordConfirm  bool
	recordConfirmDismissed bool
	recording              bool
	stopWhenMeetingEnds    bool
	meetingAbsentSince     time.Time
	recordOpInFlight       bool
	recordOpAt             time.Time
	lastAutoStopAt         time.Time
	lastMeetingSessionKey  string
	wantAutoRecordResume   bool
	// resumeForSessionKey scopes wantAutoRecordResume to the meeting that
	// granted it. Without it the flag outlived its meeting and let a later,
	// unrelated meeting skip the auto_record_requires_confirmation prompt.
	resumeForSessionKey string
	// recordingSessionKey remembers which meeting the current recording belongs
	// to, since detection has already moved on by the time an auto-stop lands.
	recordingSessionKey  string
	autoRecordRetryAfter time.Time
	autoRecordFailures   int
	statusNote           string
	statusExpiry         time.Time
	recordStart          time.Time
	sessionDir           string
	sessionID            int64 // row id of the in-flight recording; 0 when not recording
	errMsg               string
	doctorReport         doctor.Report
	sessions             []session.Record
	sessionCursor        int
	sessionsErr          string
	width                int
	height               int
	quitting             bool

	helpOpen          bool
	quitConfirmOpen   bool
	quitConfirmCursor int

	// Resolved device labels for display
	systemDevice     string
	micDevice        string
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
	levelFrame  uint64 // bumps each level tick; purely informational
	levelQuiet  int    // consecutive unchanged level reads, drives tick backoff
	idlePolls   int    // consecutive detection polls with no meeting, drives poll backoff

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

	whisperInstallActive bool
	whisperInstallLog    []string
	whisperInstallErr    string
	whisperInstallScroll int
	whisperInstallCancel context.CancelFunc

	gpuInstallActive bool
	gpuInstallLog    []string
	gpuInstallErr    string
	gpuInstallScroll int
	gpuInstallCancel context.CancelFunc

	installFrame uint64 // drives the spinner while installs run

	doctorWhisperCanInstall bool
	doctorGPUCanInstall     bool

	setupOpen    bool
	setupWizard  setup.WizardState
	setupSummary []string
	setupCancel  context.CancelFunc

	sessionsOpenerPicker  bool
	sessionsOpenerCursor  int
	sessionsDesktopNote   string
	sessionsPage          int
	sessionsDeleteConfirm bool
	sessionsDeleteCursor  int

	// scroll coalesces high-frequency wheel input; see scroll_input.go for why
	// it is a pointer.
	scroll *sessionScrollAccumulator
}

// NewModel creates the initial TUI model.
func NewModel(deps Deps) Model {
	m := Model{
		deps:       deps,
		screen:     ScreenMain,
		appState:   StateIdle,
		provider:   "none",
		autoRecord: deps.Config.AutoRecord,
		scroll:     newSessionScroll(),
		recStatus:  deps.Recorder.Status(),
	}
	if setup.NeedsSetup(deps.Config, deps.Platform) {
		m = m.openSetupWizard()
	}
	return m
}

// pollInterval is the delay before the next detection poll. Each poll forks an
// external tool (pactl, ps, xdotool, PowerShell), which is by far the most
// expensive recurring thing anoted does. Once nothing has happened for a while
// the interval stretches out; any detected meeting snaps it back immediately,
// so the only cost is a slower first detection on a long-idle machine.
func (m Model) pollInterval() time.Duration {
	base := m.pollIntervalBase()
	if m.recording || m.detection.InMeeting {
		return base
	}
	switch {
	case m.idlePolls >= idlePollsLong:
		return base * 5
	case m.idlePolls >= idlePollsShort:
		return base * 3
	default:
		return base
	}
}

func (m Model) pollIntervalBase() time.Duration {
	ms := m.deps.Config.Detection.PollIntervalMS
	if ms <= 0 {
		ms = 2000
	}
	return time.Duration(ms) * time.Millisecond
}

const (
	// ~2 min of quiet at the default 2s base interval, then ~10 min.
	idlePollsShort = 60
	idlePollsLong  = 160
)

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

// applyConfig swaps in a new configuration and rebuilds everything derived from
// it.
//
// Assigning deps.Config on its own is not enough: transcribe.Service captures
// the config it was constructed with, so switching the transcription backend in
// the Config tab wrote the new value to disk and displayed it correctly while
// transcription silently kept using the engine from startup until anoted was
// restarted. Six of the eight config-update paths had that bug.
func (m Model) applyConfig(cfg config.Config) Model {
	m.deps.Config = cfg
	m.deps.Transcriber = transcribe.New(cfg)
	return m
}

type pollTickMsg struct{}
type durationTickMsg struct{}

// trayQuitMsg is sent when the user picks Quit from the system tray. It has to
// go through Update: calling Program.Quit directly ends the event loop without
// dispatching, so the stop-recording and flush steps in performQuit are skipped.
type TrayQuitMsg struct{}
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
	sessionID    int64
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

const sessionsListLimit = 500

func loadSessionRecords(store session.Store) ([]session.Record, error) {
	if store == nil {
		return nil, nil
	}
	return store.List(sessionsListLimit)
}

// recorderUnusable reports why the active backend cannot capture audio, or ""
// when it can. Both the r key and auto-record consult it so anoted never shows
// a recording indicator for a backend that writes nothing.
func (m Model) recorderUnusable() string {
	// NewModel calls deps.Recorder.Status(), so a real Model always has one;
	// only focused unit tests construct a Model without it, and they are asking
	// about the decision logic rather than backend availability.
	if m.deps.Recorder == nil {
		return ""
	}
	return m.deps.Recorder.Unusable()
}

// sessionsPageSize is how many session rows fit on screen.
//
// It was the constant 6 regardless of terminal size, so a tall terminal wasted
// most of its height while a short one still tried to draw six rows plus the
// details and preview panels, pushing the footer off the bottom.
func (m Model) sessionsPageSize() int {
	// Rows consumed by the header, tabs, status/audio boxes, the details and
	// preview panels, borders and the footer.
	const chrome = 26
	n := m.height - chrome
	switch {
	case m.height <= 0:
		return sessionsPageSizeDefault
	case n < sessionsPageSizeMin:
		return sessionsPageSizeMin
	case n > sessionsPageSizeMax:
		return sessionsPageSizeMax
	default:
		return n
	}
}

const (
	sessionsPageSizeMin     = 3
	sessionsPageSizeMax     = 14
	sessionsPageSizeDefault = 6
)
