package transcribe

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"anoted/internal/config"
)

func TestParseVRAMLine(t *testing.T) {
	free, total, ok := parseVRAMLine("  2048, 8192 \r")
	if !ok || free != 2048 || total != 8192 {
		t.Fatalf("parse = %d %d %v", free, total, ok)
	}
	if _, _, ok := parseVRAMLine("N/A, N/A"); ok {
		t.Fatal("expected N/A to be unreadable")
	}
	if _, _, ok := parseVRAMLine(""); ok {
		t.Fatal("expected empty line to be unreadable")
	}
}

func TestModelVRAMBudgetLighterIsSmaller(t *testing.T) {
	for _, model := range []string{"tiny", "turbo", "large-v3", "unknown-model"} {
		full, light := modelVRAMBudgetMiB(model)
		if light <= 0 || light > full {
			t.Fatalf("%s budget full=%d light=%d", model, full, light)
		}
	}
	if full, _ := modelVRAMBudgetMiB("large-v3"); full != 6144 {
		t.Fatalf("large budget = %d, want 6144", full)
	}
}

// stubVRAM installs a scripted nvidia-smi and an instant pause for one test.
func stubVRAM(t *testing.T, readings []vramSnap) *int {
	t.Helper()
	origQ, origP := queryGPUMemory, gpuPause
	t.Cleanup(func() {
		queryGPUMemory = origQ
		gpuPause = origP
	})
	var i int
	queryGPUMemory = func() (int, int, bool) {
		idx := i
		if idx >= len(readings) {
			idx = len(readings) - 1
		} else {
			i++
		}
		r := readings[idx]
		return r.free, r.total, r.ok
	}
	gpuPause = func(context.Context, time.Duration) error { return nil }
	return &i
}

func cudaCfg() config.TranscriptionConfig {
	return config.TranscriptionConfig{Device: DeviceCUDA, Model: "turbo"}
}

func TestGuardSkipsWaitOnCPU(t *testing.T) {
	orig := queryGPUMemory
	t.Cleanup(func() { queryGPUMemory = orig })
	queryGPUMemory = func() (int, int, bool) {
		t.Fatal("CPU transcription should not probe VRAM")
		return 0, 0, false
	}
	calls := 0
	err := transcribeGuardingVRAM(context.Background(), config.TranscriptionConfig{Device: DeviceCPU, Model: "turbo"}, nil,
		func(context.Context) error {
			calls++
			return nil
		})
	if err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("calls = %d, want 1", calls)
	}
}

func TestGuardRunsFullQualityWhenVRAMIsFree(t *testing.T) {
	stubVRAM(t, []vramSnap{{free: 7000, total: 8192, ok: true}})
	var mode gpuMode
	err := transcribeGuardingVRAM(context.Background(), cudaCfg(), nil, func(ctx context.Context) error {
		mode = gpuModeFrom(ctx)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if mode != gpuModeNormal {
		t.Fatalf("mode = %d, want normal", mode)
	}
}

func TestGuardUsesLighterModeWhenGPUIsTight(t *testing.T) {
	// turbo wants ~4 GB for full quality and ~2.5 GB for the lighter pass.
	// 3 GB free is enough to share the card, not enough for float16.
	stubVRAM(t, []vramSnap{{free: 3072, total: 8192, ok: true}})
	var lines []string
	var mode gpuMode
	err := transcribeGuardingVRAM(context.Background(), cudaCfg(), func(p Progress) {
		if p.SegmentText != "" {
			lines = append(lines, p.SegmentText)
		}
	}, func(ctx context.Context) error {
		mode = gpuModeFrom(ctx)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if mode != gpuModeLight {
		t.Fatalf("mode = %d, want light", mode)
	}
	if len(lines) != 1 || !strings.Contains(lines[0], "lighter, slower") {
		t.Fatalf("lines = %v", lines)
	}
}

func TestGuardQueuesUntilVRAMFrees(t *testing.T) {
	// First look is a full card; after the poll the game has exited.
	stubVRAM(t, []vramSnap{
		{free: 400, total: 8192, ok: true},
		{free: 7000, total: 8192, ok: true},
	})
	var lines []string
	calls := 0
	err := transcribeGuardingVRAM(context.Background(), cudaCfg(), func(p Progress) {
		if p.SegmentText != "" {
			lines = append(lines, p.SegmentText)
		}
	}, func(ctx context.Context) error {
		calls++
		if gpuModeFrom(ctx) != gpuModeNormal {
			t.Fatalf("mode = %d, want normal once the card is free", gpuModeFrom(ctx))
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("run calls = %d, want 1 (must not start while VRAM is short)", calls)
	}
	if len(lines) == 0 || !strings.Contains(lines[0], "queued") {
		t.Fatalf("lines = %v", lines)
	}
}

func TestGuardRetriesOOMWhenGPUIsBusy(t *testing.T) {
	// 3 GB free: start lighter, OOM, then the card frees up past the bumped bar.
	stubVRAM(t, []vramSnap{
		{free: 3072, total: 8192, ok: true}, // start light
		{free: 3072, total: 8192, ok: true}, // after OOM, still contested
		{free: 3072, total: 8192, ok: true}, // queue poll, under minFree
		{free: 7000, total: 8192, ok: true}, // game closed
	})
	calls := 0
	err := transcribeGuardingVRAM(context.Background(), cudaCfg(), nil, func(ctx context.Context) error {
		calls++
		if calls == 1 {
			return errors.New("CUDA failed with error out of memory")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if calls != 2 {
		t.Fatalf("calls = %d, want 2", calls)
	}
}

func TestGuardGivesUpWhenIdleGPUStillOOMs(t *testing.T) {
	// 7 GB free on an 8 GB card: the model itself does not fit. One lighter
	// retry, then the error — do not queue forever.
	stubVRAM(t, []vramSnap{{free: 7000, total: 8192, ok: true}})
	calls := 0
	err := transcribeGuardingVRAM(context.Background(), cudaCfg(), nil, func(context.Context) error {
		calls++
		return errors.New("CUDA failed with error out of memory")
	})
	if err == nil || !isCUDAOOM(err.Error()) {
		t.Fatalf("err = %v", err)
	}
	if !strings.Contains(err.Error(), "close other apps") {
		t.Fatalf("err = %v, want the give-up hint", err)
	}
	if calls != 2 {
		t.Fatalf("calls = %d, want 2", calls)
	}
}

func TestGuardDoesNotRetryOrdinaryErrors(t *testing.T) {
	stubVRAM(t, []vramSnap{{free: 7000, total: 8192, ok: true}})
	calls := 0
	err := transcribeGuardingVRAM(context.Background(), cudaCfg(), nil, func(context.Context) error {
		calls++
		return errors.New("audio file missing")
	})
	if err == nil || calls != 1 {
		t.Fatalf("calls=%d err=%v", calls, err)
	}
}

func TestGuardCancelWhileQueued(t *testing.T) {
	origQ, origP := queryGPUMemory, gpuPause
	t.Cleanup(func() {
		queryGPUMemory = origQ
		gpuPause = origP
	})
	queryGPUMemory = func() (int, int, bool) { return 200, 8192, true }
	ctx, cancel := context.WithCancel(context.Background())
	gpuPause = func(ctx context.Context, _ time.Duration) error {
		cancel()
		return ctx.Err()
	}
	calls := 0
	err := transcribeGuardingVRAM(ctx, cudaCfg(), nil, func(context.Context) error {
		calls++
		return nil
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want canceled", err)
	}
	if calls != 0 {
		t.Fatalf("calls = %d, want 0", calls)
	}
}
