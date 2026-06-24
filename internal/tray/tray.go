package tray

// State is the tray icon mode shown to the user.
type State int

const (
	StateMonitoring State = iota
	StateRecording
)

// Indicator shows app status in the system notification area.
type Indicator interface {
	Start() error
	SetState(State)
	SetTooltip(string)
	OnQuit(func())
	Stop()
}

// Options configures the tray indicator.
type Options struct {
	Enabled      bool
	OnOpenFolder func() error
	OnQuit       func()
}

// New returns a platform tray indicator or a no-op stub.
func New(opts Options) Indicator {
	if !opts.Enabled {
		return noopIndicator{}
	}
	return newPlatformIndicator(opts)
}

type noopIndicator struct{}

func (noopIndicator) Start() error              { return nil }
func (noopIndicator) SetState(State)          {}
func (noopIndicator) SetTooltip(string)       {}
func (noopIndicator) OnQuit(func())           {}
func (noopIndicator) Stop()                   {}
