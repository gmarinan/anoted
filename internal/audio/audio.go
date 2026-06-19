package audio

// Kind identifies an audio capture device type.
type Kind string

const (
	KindOutput Kind = "output"
	KindMic    Kind = "microphone"
)

// Device is a selectable PipeWire/PulseAudio node.
type Device struct {
	// ID is the value stored in config: sink.monitor for outputs, source name for mics.
	ID string
	// Name is the PipeWire node.name (sink or source).
	Name string
	// State is RUNNING, SUSPENDED, IDLE, etc.
	State string
	// NodeID is the Pulse/PipeWire object index shown in pw-top.
	NodeID string
	// Format describes sample format/channels/rate when available.
	Format string
	// IsDefault marks the system default sink or source.
	IsDefault bool
	// LinkedApps lists clients currently playing to this sink (outputs only).
	LinkedApps []string
}

// Catalog lists available audio devices on the host.
type Catalog struct {
	Outputs     []Device // playback sinks; recording uses sink.monitor
	Microphones []Device // capture sources
}

// Provider lists audio devices for the current platform.
type Provider interface {
	List() (Catalog, error)
	Resolve(systemMonitor, microphone string) (resolvedSystem, resolvedMic string, err error)
	MonitorWarning(configuredMonitor string) string
}

// NewProvider returns the platform-specific device provider.
func NewProvider() Provider {
	return newProvider()
}

// AutoLabel is the display name for automatic device selection.
const AutoLabel = "(auto: system default)"

// AutoID means follow the OS default sink monitor / default source.
const AutoID = ""
