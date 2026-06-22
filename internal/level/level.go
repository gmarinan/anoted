package level

// LevelSnapshot holds normalized peaks and per-band spectrum levels (0..1).
type LevelSnapshot struct {
	System      float64
	Mic         float64
	SystemBands []float64 // len BandCount, fixed frequency bars L→R
	MicBands    []float64
}

// Monitor reads live audio levels from system output and microphone sources.
type Monitor interface {
	Available() bool
	SetStreamOptions(latencyMsec, processTimeMsec int)
	StartSystem(monitorID string) error
	StartMic(sourceID string) error
	StopSystem() error
	StopMic() error
	Read() LevelSnapshot
	Close() error
}

// NewMonitor returns a platform-specific level monitor.
func NewMonitor(resolver DeviceResolver) Monitor {
	return newMonitor(resolver)
}

// DeviceResolver resolves configured device IDs to Pulse/PipeWire names.
type DeviceResolver interface {
	Resolve(systemMonitor, microphone string) (resolvedSystem, resolvedMic string, err error)
}
