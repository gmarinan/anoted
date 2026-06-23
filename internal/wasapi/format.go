package wasapi

const (
	// CanonicalSampleRate is the shared capture and WAV output sample rate.
	CanonicalSampleRate = 48000
	// CanonicalChannels is the shared capture and WAV output channel count.
	CanonicalChannels = 2
)

// CanonicalFormat returns the capture format, defaulting to 48 kHz stereo.
func CanonicalFormat(sampleRate, channels int) (uint32, uint32) {
	if sampleRate <= 0 {
		sampleRate = CanonicalSampleRate
	}
	if channels <= 0 {
		channels = CanonicalChannels
	}
	return uint32(sampleRate), uint32(channels)
}
