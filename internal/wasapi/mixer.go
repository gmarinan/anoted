package wasapi

import (
	"time"
)

// MixS16 averages two s16le PCM buffers frame-by-frame into dst.
// Shorter input is treated as silence. dst must be large enough for the longest input.
func MixS16(dst, a, b []byte) []byte {
	n := len(a)
	if len(b) > n {
		n = len(b)
	}
	if cap(dst) < n {
		dst = make([]byte, n)
	} else {
		dst = dst[:n]
	}
	for i := 0; i < n; i += 2 {
		var av, bv int32
		if i+1 < len(a) {
			av = int32(int16(int(a[i]) | int(a[i+1])<<8))
		}
		if i+1 < len(b) {
			bv = int32(int16(int(b[i]) | int(b[i+1])<<8))
		}
		mixed := (av + bv) / 2
		if mixed > 32767 {
			mixed = 32767
		}
		if mixed < -32768 {
			mixed = -32768
		}
		dst[i] = byte(mixed)
		dst[i+1] = byte(mixed >> 8)
	}
	return dst
}

// DownmixToMono copies the first channel of interleaved s16le PCM into mono dst.
func DownmixToMono(dst, interleaved []byte, channels int) []byte {
	if channels <= 1 {
		if cap(dst) < len(interleaved) {
			dst = make([]byte, len(interleaved))
		} else {
			dst = dst[:len(interleaved)]
		}
		copy(dst, interleaved)
		return dst
	}
	frames := len(interleaved) / (2 * channels)
	need := frames * 2
	if cap(dst) < need {
		dst = make([]byte, need)
	} else {
		dst = dst[:need]
	}
	for i := 0; i < frames; i++ {
		off := i * 2 * channels
		dst[i*2] = interleaved[off]
		dst[i*2+1] = interleaved[off+1]
	}
	return dst
}

const (
	defaultMicFIFOms = 200
	mixerTickMs      = 10
)

// MasterClockMixer mixes loopback and microphone PCM on a steady real-time clock.
type MasterClockMixer struct {
	sampleRate      int
	channels        int
	frameBytes      int
	maxMicFIFO      int
	maxLoopFIFO     int
	pendingLoop     []byte
	micFIFO         []byte
	silentLoopFrame []byte
	mixBuf          []byte
	onPCM           func([]byte)
	outputFrames    int

	// now is injectable so tests can drive the clock; lastEmit tracks how much
	// audio time has already been written.
	now      func() time.Time
	lastEmit time.Time
}

// NewMasterClockMixer creates a mixer driven by EmitTicks.
func NewMasterClockMixer(sampleRate, channels int, onPCM func([]byte)) *MasterClockMixer {
	if sampleRate <= 0 {
		sampleRate = CanonicalSampleRate
	}
	if channels <= 0 {
		channels = CanonicalChannels
	}
	frameBytes := channels * 2
	maxFrames := sampleRate * defaultMicFIFOms / 1000
	if maxFrames < 1 {
		maxFrames = 1
	}
	return &MasterClockMixer{
		sampleRate:      sampleRate,
		channels:        channels,
		frameBytes:      frameBytes,
		maxMicFIFO:      maxFrames * frameBytes,
		maxLoopFIFO:     maxFrames * frameBytes,
		silentLoopFrame: make([]byte, frameBytes),
		onPCM:           onPCM,
		now:             time.Now,
	}
}

// TickInterval returns how often the mixer should emit a batch of frames.
func (m *MasterClockMixer) TickInterval() time.Duration {
	return time.Duration(mixerTickMs) * time.Millisecond
}

// FramesPerTick returns how many frames to emit per tick.
func (m *MasterClockMixer) FramesPerTick() int {
	frames := m.sampleRate * mixerTickMs / 1000
	if frames < 1 {
		return 1
	}
	return frames
}

// PushLoop buffers loopback PCM for the next ticks.
func (m *MasterClockMixer) PushLoop(chunk []byte) {
	if len(chunk) == 0 {
		return
	}
	m.pendingLoop = append(m.pendingLoop, chunk...)
	m.trimLoopFIFO()
}

// PushMic buffers microphone PCM for the next ticks.
func (m *MasterClockMixer) PushMic(chunk []byte) {
	if len(chunk) == 0 {
		return
	}
	m.micFIFO = append(m.micFIFO, chunk...)
	m.trimMicFIFO()
}

// EmitTicks writes as many frames as real time has elapsed for since the last
// call.
//
// Emitting a fixed FramesPerTick() per received tick lost time in one direction
// only: time.Ticker drops ticks when its receiver is busy, and the sole
// receiver also runs every OnPCM callback plus the WAV writes. Missed ticks
// were never made up, so a long recording drifted progressively shorter than
// the meeting it captured.
func (m *MasterClockMixer) EmitTicks() {
	now := m.now()
	if m.lastEmit.IsZero() {
		m.lastEmit = now
		m.emitFrames(m.FramesPerTick())
		return
	}

	elapsed := now.Sub(m.lastEmit)
	if elapsed <= 0 {
		return
	}
	frames := int(float64(elapsed) / float64(time.Second) * float64(m.sampleRate))
	if frames < 1 {
		return
	}
	// Bound a single catch-up burst (after a suspend, say) to one second, and
	// advance the clock only by what was actually written so the remainder is
	// made up on subsequent ticks instead of being dropped.
	if maxCatchUp := m.sampleRate; frames > maxCatchUp {
		frames = maxCatchUp
	}
	m.lastEmit = m.lastEmit.Add(time.Duration(float64(frames) / float64(m.sampleRate) * float64(time.Second)))
	m.emitFrames(frames)
}

func (m *MasterClockMixer) emitFrames(n int) {
	for i := 0; i < n; i++ {
		m.emitTick()
	}
}

// OutputFrames returns the number of mixed frames emitted.
func (m *MasterClockMixer) OutputFrames() int {
	return m.outputFrames
}

func (m *MasterClockMixer) trimMicFIFO() {
	if len(m.micFIFO) <= m.maxMicFIFO {
		return
	}
	drop := len(m.micFIFO) - m.maxMicFIFO
	m.micFIFO = m.micFIFO[drop:]
}

func (m *MasterClockMixer) trimLoopFIFO() {
	if len(m.pendingLoop) <= m.maxLoopFIFO {
		return
	}
	drop := len(m.pendingLoop) - m.maxLoopFIFO
	m.pendingLoop = m.pendingLoop[drop:]
}

func (m *MasterClockMixer) emitTick() {
	var loopFrame, micFrame []byte
	if len(m.pendingLoop) >= m.frameBytes {
		loopFrame = m.pendingLoop[:m.frameBytes]
		m.pendingLoop = m.pendingLoop[m.frameBytes:]
	} else {
		loopFrame = m.silentLoopFrame
	}
	if len(m.micFIFO) >= m.frameBytes {
		micFrame = m.micFIFO[:m.frameBytes]
		m.micFIFO = m.micFIFO[m.frameBytes:]
	}
	m.mixBuf = MixS16(m.mixBuf[:0], loopFrame, micFrame)
	if m.onPCM != nil {
		m.onPCM(m.mixBuf)
	}
	m.outputFrames++
}
