package wasapi

import (
	"testing"
	"time"
)

func TestMixS16(t *testing.T) {
	a := []byte{0, 0, 10, 0}
	b := []byte{0, 0, 20, 0}
	out := MixS16(nil, a, b)
	if len(out) != 4 {
		t.Fatalf("len %d", len(out))
	}
	if out[2] != 15 || out[3] != 0 {
		t.Fatalf("mixed sample: %v", out)
	}
}

func TestMasterClockMixerLoopDriven(t *testing.T) {
	var out [][]byte
	mixer := NewMasterClockMixer(CanonicalSampleRate, CanonicalChannels, func(pcm []byte) {
		frame := make([]byte, len(pcm))
		copy(frame, pcm)
		out = append(out, frame)
	})

	loop := make([]byte, 8) // 2 stereo frames
	loop[2], loop[3] = 20, 0
	loop[6], loop[7] = 40, 0
	mixer.PushLoop(loop)
	mixer.emitFrames(1)
	mixer.emitFrames(1)

	if got := mixer.OutputFrames(); got != 2 {
		t.Fatalf("output frames: got %d want 2", got)
	}
	if out[0][2] != 10 || out[0][3] != 0 {
		t.Fatalf("first mixed frame: %v", out[0])
	}
	if out[1][2] != 20 || out[1][3] != 0 {
		t.Fatalf("second mixed frame: %v", out[1])
	}
}

func TestMasterClockMixerLoopUnderrunPadsSilence(t *testing.T) {
	var out []byte
	mixer := NewMasterClockMixer(CanonicalSampleRate, CanonicalChannels, func(pcm []byte) {
		out = append(out, pcm...)
	})

	loop := []byte{0, 0, 100, 0}
	mixer.PushLoop(loop)
	mixer.emitFrames(1)

	if len(out) != 4 {
		t.Fatalf("len %d", len(out))
	}
	if out[2] != 50 || out[3] != 0 {
		t.Fatalf("loop-only mix: %v", out)
	}
}

func TestMasterClockMixerMicOnlyEmits(t *testing.T) {
	var out []byte
	mixer := NewMasterClockMixer(CanonicalSampleRate, CanonicalChannels, func(pcm []byte) {
		out = append(out, pcm...)
	})

	mic := []byte{0, 0, 20, 0, 0, 0, 40, 0}
	mixer.PushMic(mic)
	mixer.emitFrames(1)
	mixer.emitFrames(1)

	if got := mixer.OutputFrames(); got != 2 {
		t.Fatalf("output frames: got %d want 2", got)
	}
	if out[2] != 10 || out[3] != 0 {
		t.Fatalf("first mic-only frame: %v", out[:4])
	}
	if out[6] != 20 || out[7] != 0 {
		t.Fatalf("second mic-only frame: %v", out[4:8])
	}
}

func TestMasterClockMixerMicFIFOBounded(t *testing.T) {
	mixer := NewMasterClockMixer(CanonicalSampleRate, CanonicalChannels, nil)
	frameBytes := CanonicalChannels * 2
	maxBytes := CanonicalSampleRate * defaultMicFIFOms / 1000 * frameBytes

	mic := make([]byte, maxBytes+frameBytes*4)
	mixer.PushMic(mic)

	if len(mixer.micFIFO) > maxBytes {
		t.Fatalf("mic fifo %d exceeds max %d", len(mixer.micFIFO), maxBytes)
	}
}

func TestMasterClockMixerFramesPerTick(t *testing.T) {
	mixer := NewMasterClockMixer(CanonicalSampleRate, CanonicalChannels, nil)
	if got := mixer.FramesPerTick(); got != 480 {
		t.Fatalf("frames per tick: got %d want 480", got)
	}
}

func TestMasterClockMixerSteadyTickCount(t *testing.T) {
	mixer := NewMasterClockMixer(CanonicalSampleRate, CanonicalChannels, nil)
	clock := time.Unix(0, 0)
	mixer.now = func() time.Time { return clock }

	for i := 0; i < 100; i++ {
		clock = clock.Add(mixer.TickInterval())
		mixer.EmitTicks()
	}
	want := 100 * mixer.FramesPerTick()
	if got := mixer.OutputFrames(); got != want {
		t.Fatalf("output frames: got %d want %d", got, want)
	}
}

func TestMasterClockMixerMakesUpDroppedTicks(t *testing.T) {
	// time.Ticker drops ticks when its receiver is busy, and the sole receiver
	// also runs every OnPCM callback. Emitting a fixed count per received tick
	// therefore lost audio time one way, shortening long recordings.
	mixer := NewMasterClockMixer(CanonicalSampleRate, CanonicalChannels, nil)
	clock := time.Unix(0, 0)
	mixer.now = func() time.Time { return clock }

	// First tick establishes the baseline.
	clock = clock.Add(mixer.TickInterval())
	mixer.EmitTicks()

	// One second of real time passes but only a single tick is delivered.
	clock = clock.Add(time.Second)
	mixer.EmitTicks()

	// Output must cover the elapsed second, not just one 10ms tick.
	want := mixer.FramesPerTick() + CanonicalSampleRate
	if got := mixer.OutputFrames(); got != want {
		t.Fatalf("dropped ticks were not made up: got %d frames, want %d", got, want)
	}
}

func TestMasterClockMixerBatchMixesAndPadsInOneCall(t *testing.T) {
	var calls [][]byte
	mixer := NewMasterClockMixer(CanonicalSampleRate, CanonicalChannels, func(pcm []byte) {
		calls = append(calls, append([]byte(nil), pcm...))
	})

	loop := make([]byte, 8) // 2 stereo frames
	loop[2] = 20
	loop[6] = 40
	mixer.PushLoop(loop)
	mixer.emitFrames(3) // one more frame than buffered: the tail must be silence

	if len(calls) != 1 {
		t.Fatalf("a batch should reach onPCM once, got %d calls", len(calls))
	}
	out := calls[0]
	if len(out) != 12 {
		t.Fatalf("len %d want 12", len(out))
	}
	if out[2] != 10 || out[6] != 20 {
		t.Fatalf("mixed frames: %v", out)
	}
	for i, b := range out[8:] {
		if b != 0 {
			t.Fatalf("tail byte %d should be silence: %v", i, out[8:])
		}
	}
	if got := mixer.OutputFrames(); got != 3 {
		t.Fatalf("output frames: got %d want 3", got)
	}
}
