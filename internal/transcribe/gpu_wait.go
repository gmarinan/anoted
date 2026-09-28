package transcribe

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"anoted/internal/config"
)

const (
	// vramReclaimPause gives the driver a moment to take back the allocation
	// from a process that just died of OOM. Sampling sooner still shows the
	// dead job's memory and we queue even though the card is free.
	vramReclaimPause = 3 * time.Second
	// vramPollInterval is how often a queued transcription looks again. Short
	// enough that closing the game is noticed quickly, long enough that
	// nvidia-smi is not polled in a tight loop.
	vramPollInterval = 10 * time.Second
)

// gpuMode is how hard this attempt is allowed to push the card.
type gpuMode int

const (
	gpuModeNormal gpuMode = iota
	gpuModeLight
)

type gpuModeKey struct{}

func withGPUMode(ctx context.Context, mode gpuMode) context.Context {
	return context.WithValue(ctx, gpuModeKey{}, mode)
}

func gpuModeFrom(ctx context.Context) gpuMode {
	mode, _ := ctx.Value(gpuModeKey{}).(gpuMode)
	return mode
}

// queryGPUMemory is swapped in tests. Production reads nvidia-smi.
var queryGPUMemory = queryGPUMemorySMI

// gpuPause is swapped in tests so a queued run does not actually sleep.
var gpuPause = func(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

type vramSnap struct {
	free  int
	total int
	ok    bool
}

func readVRAM() vramSnap {
	free, total, ok := queryGPUMemory()
	return vramSnap{free: free, total: total, ok: ok}
}

// contested means the card does not currently have the headroom the full
// quality pass asked for. That is the "a game has the GPU" case: retrying
// immediately would just crash again.
func (s vramSnap) contested(full int) bool {
	if !s.ok || s.total <= 0 {
		return false
	}
	return s.free < full
}

// modelVRAMBudgetMiB is a conservative free-VRAM bar for one model, in MiB.
//
// The numbers are weights plus the workspace a transcribe pass actually
// allocates, not the file size on disk. They are deliberately a little high:
// starting when the card is short crashes the job, and waiting a bit longer
// does not. The lighter bar is what int8_float16 / a partial GPU offload needs.
func modelVRAMBudgetMiB(model string) (full, light int) {
	switch strings.ToLower(strings.TrimSpace(model)) {
	case "tiny", "tiny.en":
		return 1024, 768
	case "base", "base.en":
		return 1280, 896
	case "small", "small.en", "distil-small.en":
		return 2048, 1280
	case "medium", "medium.en", "distil-medium.en":
		return 4096, 2560
	case "turbo", "large-v3-turbo":
		return 4096, 2560
	case "large", "large-v1", "large-v2", "large-v3":
		return 6144, 4096
	default:
		return 4096, 2560
	}
}

func parseVRAMLine(line string) (free, total int, ok bool) {
	line = strings.TrimSpace(line)
	parts := strings.Split(line, ",")
	if len(parts) < 2 {
		return 0, 0, false
	}
	var err1, err2 error
	free, err1 = strconv.Atoi(strings.TrimSpace(parts[0]))
	total, err2 = strconv.Atoi(strings.TrimSpace(parts[1]))
	if err1 != nil || err2 != nil || total <= 0 || free < 0 {
		return 0, 0, false
	}
	return free, total, true
}

func formatGiB(mib int) string {
	gb := float64(mib) / 1024
	if gb < 10 {
		return fmt.Sprintf("%.1f GB", gb)
	}
	return fmt.Sprintf("%.0f GB", gb)
}

// transcribeGuardingVRAM runs a CUDA transcription without treating a busy
// GPU as a hard failure.
//
// A game sitting on the card makes whisper abort with "CUDA out of memory".
// Falling straight through to CPU would pin the machine for the whole meeting,
// and surfacing the error drops the transcript on the floor. Instead: if there
// is not enough free VRAM, wait (the user can cancel with s); if there is some
// room but not enough for full quality, run a lighter, slower pass that can
// share the card; if the GPU is actually idle and it still dies, stop.
func transcribeGuardingVRAM(ctx context.Context, cfg config.TranscriptionConfig, onProgress ProgressFunc, run func(context.Context) error) error {
	if resolveDevice(cfg) != DeviceCUDA {
		return run(ctx)
	}
	full, light := modelVRAMBudgetMiB(resolvedModel(cfg))
	var (
		forceLight bool
		minFree    int
		stalls     int
		abrupt     int
	)
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		mode, err := waitForGPUSlot(ctx, full, light, minFree, forceLight, onProgress)
		if err != nil {
			return err
		}
		// forceLight applies to this attempt only. If the game has quit by the
		// next pass, full quality is available again.
		forceLight = false
		err = run(withGPUMode(ctx, mode))
		if err == nil {
			return nil
		}
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return err
		}
		oom, crashed := classifyGPUFailure(err)
		if !oom && !crashed {
			return err
		}
		if crashed {
			abrupt++
			// An abort with no OOM text is usually a real crash. Retry a couple
			// of times in case the driver killed the process for VRAM, then stop.
			if abrupt >= 3 {
				return err
			}
		}
		if err := gpuPause(ctx, vramReclaimPause); err != nil {
			return err
		}
		snap := readVRAM()
		if snap.contested(full) {
			stalls = 0
			if snap.ok {
				bump := snap.free + 512
				if snap.total > 768 && bump > snap.total-768 {
					bump = snap.total - 768
				}
				if bump > minFree {
					minFree = bump
				}
			}
			emitProgress(onProgress, Progress{SegmentText: "· GPU out of memory — queued until VRAM frees up"})
			slog.Warn("transcription out of GPU memory; waiting to retry",
				"free_mib", snap.free, "total_mib", snap.total, "need_mib", minFree)
			continue
		}
		stalls++
		if stalls == 1 {
			forceLight = true
			emitProgress(onProgress, Progress{SegmentText: "· GPU out of memory — retrying slower so it can share the GPU"})
			slog.Warn("transcription out of GPU memory; retrying lighter",
				"free_mib", snap.free, "total_mib", snap.total)
			continue
		}
		if oom {
			return terminalVRAMError(err)
		}
		return err
	}
}

func classifyGPUFailure(err error) (oom, abrupt bool) {
	if err == nil {
		return false, false
	}
	msg := err.Error()
	if isCUDAOOM(msg) {
		return true, false
	}
	ls := strings.ToLower(msg)
	if strings.Contains(ls, "signal: killed") || strings.Contains(ls, "signal: aborted") ||
		strings.Contains(ls, "exit status 137") || strings.Contains(ls, "exit status 134") {
		return false, true
	}
	return false, false
}

func terminalVRAMError(err error) error {
	if strings.Contains(strings.ToLower(err.Error()), "close other apps using the gpu") {
		return err
	}
	return fmt.Errorf("%w — close other apps using the GPU, try a smaller model, or set transcription.device: cpu", err)
}

// waitForGPUSlot blocks until the card can take this attempt, or until ctx is
// cancelled. When nvidia-smi cannot be read, it does not block: there is
// nothing to wait on.
func waitForGPUSlot(ctx context.Context, full, light, minFree int, forceLight bool, onProgress ProgressFunc) (gpuMode, error) {
	sawReading := false
	misses := 0
	announced := false
	var lastLine string
	var lastEmit time.Time
	for {
		if err := ctx.Err(); err != nil {
			return 0, err
		}
		snap := readVRAM()
		if !snap.ok {
			if !sawReading {
				return modeWithoutReading(forceLight), nil
			}
			misses++
			if misses >= 3 {
				return modeWithoutReading(forceLight), nil
			}
			if err := gpuPause(ctx, vramPollInterval); err != nil {
				return 0, err
			}
			continue
		}
		sawReading = true
		misses = 0

		if !forceLight && snap.free >= full && snap.free >= minFree {
			return gpuModeNormal, nil
		}
		if snap.free >= light && snap.free >= minFree {
			// Room for the small pass but not the full one: run now, slower,
			// instead of crashing or sitting in the queue until the game exits.
			if !forceLight && snap.free < full {
				emitProgress(onProgress, Progress{SegmentText: "· GPU memory is tight — using a lighter, slower mode"})
				slog.Info("transcription using lighter GPU mode", "free_mib", snap.free, "full_mib", full)
			}
			return gpuModeLight, nil
		}

		need := light
		if minFree > need {
			need = minFree
		}
		line := vramWaitLine(snap.free, need, !announced)
		// The same reading every 10s would flood the preview. Repeat only when
		// the numbers change, or once a minute so a long queue still looks alive.
		if line != lastLine || time.Since(lastEmit) >= time.Minute {
			emitProgress(onProgress, Progress{SegmentText: line})
			if !announced {
				slog.Info("transcription queued for GPU memory", "free_mib", snap.free, "need_mib", need)
			}
			lastLine = line
			lastEmit = time.Now()
			announced = true
		}
		if err := gpuPause(ctx, vramPollInterval); err != nil {
			return 0, err
		}
	}
}

func modeWithoutReading(forceLight bool) gpuMode {
	if forceLight {
		return gpuModeLight
	}
	return gpuModeNormal
}

func vramWaitLine(free, need int, first bool) string {
	if first {
		return fmt.Sprintf("· GPU busy (%s free, need ~%s) — queued until VRAM frees up", formatGiB(free), formatGiB(need))
	}
	return fmt.Sprintf("· still waiting for the GPU (%s free, need ~%s)", formatGiB(free), formatGiB(need))
}
