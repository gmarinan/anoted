package transcribe

import (
	"bufio"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"anoted/internal/config"
)

//go:embed faster_whisper_runner.py
var fasterWhisperRunner string

// runnerEvent is one JSON line emitted by faster_whisper_runner.py.
type runnerEvent struct {
	Type     string  `json:"type"`
	Duration float64 `json:"duration"`
	Language string  `json:"language"`
	Start    float64 `json:"start"`
	End      float64 `json:"end"`
	Text     string  `json:"text"`
	Message  string  `json:"message"`
}

// FasterWhisperPython returns the interpreter that owns the faster-whisper
// install, or "" when it is not available.
func FasterWhisperPython(cfg config.TranscriptionConfig) string {
	for _, py := range candidateFasterWhisperPythons(cfg) {
		if py == "" {
			continue
		}
		if _, err := os.Stat(py); err != nil {
			continue
		}
		if fasterWhisperImportable(py) {
			return py
		}
	}
	return ""
}

func candidateFasterWhisperPythons(cfg config.TranscriptionConfig) []string {
	var out []string
	// A configured binary usually points into a venv; its sibling python is the
	// interpreter that would have faster-whisper installed alongside it.
	if cfg.Binary != "" {
		if venv := venvDirForBinary(cfg.Binary); venv != "" {
			out = append(out, venvPythonPath(venv))
		}
	}
	out = append(out, venvPythonPath(resolveManagedVenvDir()))
	return out
}

// venvDirForBinary maps <venv>/bin/whisper back to <venv>.
func venvDirForBinary(bin string) string {
	dir := strings.TrimSuffix(bin, "/")
	for i := 0; i < 2; i++ { // strip the file, then the bin/ (or Scripts/) dir
		idx := strings.LastIndexAny(dir, `/\`)
		if idx < 0 {
			return ""
		}
		dir = dir[:idx]
	}
	return dir
}

var fasterWhisperProbe struct {
	sync.Mutex
	cache map[string]bool
}

// fasterWhisperImportable spawns the interpreter once per path and caches the
// answer; this is called from doctor and from backend resolution.
func fasterWhisperImportable(python string) bool {
	fasterWhisperProbe.Lock()
	if v, ok := fasterWhisperProbe.cache[python]; ok {
		fasterWhisperProbe.Unlock()
		return v
	}
	fasterWhisperProbe.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	err := exec.CommandContext(ctx, python, "-c", "import faster_whisper").Run()
	ok := err == nil

	fasterWhisperProbe.Lock()
	if fasterWhisperProbe.cache == nil {
		fasterWhisperProbe.cache = map[string]bool{}
	}
	fasterWhisperProbe.cache[python] = ok
	fasterWhisperProbe.Unlock()
	return ok
}

// InvalidateFasterWhisperCache forgets probe results after an install.
func InvalidateFasterWhisperCache() {
	fasterWhisperProbe.Lock()
	fasterWhisperProbe.cache = nil
	fasterWhisperProbe.Unlock()
}

// fasterWhisperComputeType picks the numeric precision for the device.
//
// float16 on GPU is the same arithmetic openai-whisper already uses, so output
// is equivalent; int8 on CPU trades a small accuracy loss on difficult audio
// for a large speedup, which is the right default when there is no GPU anyway.
func fasterWhisperComputeType(device string) string {
	if device == DeviceCUDA {
		return "float16"
	}
	return "int8"
}

func transcribeFasterWhisper(ctx context.Context, cfg config.TranscriptionConfig, python, audioPath, outDir, sessionDir string, onProgress ProgressFunc) (Result, error) {
	device := resolveDevice(cfg)
	segs, language, err := runFasterWhisper(ctx, cfg, python, audioPath, device, onProgress)
	if err != nil {
		return Result{}, err
	}

	fileBase := outputFileBase(cfg, sessionDir)
	formats := config.NormalizeOutputFormats(cfg.OutputFormats)
	files, err := writeSegmentFiles(outDir, fileBase, segs, formats, language)
	if err != nil {
		return Result{}, err
	}
	if len(files) == 0 {
		return Result{}, fmt.Errorf("faster-whisper produced no output files")
	}
	return Result{SessionDir: outDir, Files: files}, nil
}

func runFasterWhisper(ctx context.Context, cfg config.TranscriptionConfig, python, audioPath, device string, onProgress ProgressFunc) ([]Segment, string, error) {
	args := []string{
		"-", // read the runner from stdin: no temp file to create, race or clean up
		"--audio", audioPath,
		"--model", resolvedModel(cfg),
		"--device", device,
		"--compute-type", fasterWhisperComputeType(device),
	}
	if lang := strings.TrimSpace(cfg.Language); lang != "" {
		args = append(args, "--language", lang)
	}

	cmd := exec.CommandContext(ctx, python, args...)
	cmd.Stdin = strings.NewReader(fasterWhisperRunner)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, "", err
	}
	var stderr strings.Builder
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		return nil, "", fmt.Errorf("start faster-whisper: %w", err)
	}

	segs, language, runnerErr, scanErr := parseRunnerStream(stdout, onProgress)
	waitErr := cmd.Wait()

	if ctx.Err() != nil {
		return nil, "", ctx.Err()
	}
	if runnerErr != "" {
		return nil, "", fmt.Errorf("faster-whisper: %s", runnerErr)
	}
	if waitErr != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = waitErr.Error()
		}
		return nil, "", fmt.Errorf("faster-whisper: %s", trimOutput([]byte(msg)))
	}
	if scanErr != nil {
		return nil, "", fmt.Errorf("read faster-whisper output: %w", scanErr)
	}
	return segs, language, nil
}

// parseRunnerStream consumes the runner's JSON lines, reporting progress as
// segments arrive. Progress here is exact rather than estimated: the runner
// sends the audio duration before any segment, so percent is end/duration.
//
// It returns the runner's own error message separately from a read error, so
// the caller can tell "the model failed" from "the pipe broke".
func parseRunnerStream(r io.Reader, onProgress ProgressFunc) (segs []Segment, language, runnerErr string, scanErr error) {
	var duration time.Duration
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var ev runnerEvent
		if err := json.Unmarshal([]byte(line), &ev); err != nil {
			// Anything the runner did not emit as JSON is diagnostic noise.
			continue
		}
		switch ev.Type {
		case "info":
			duration = time.Duration(ev.Duration * float64(time.Second))
			language = ev.Language
			emitProgress(onProgress, Progress{AudioDuration: duration})
		case "segment":
			segs = append(segs, Segment{Start: ev.Start, End: ev.End, Text: ev.Text})
			var percent float64
			if duration > 0 {
				percent = ev.End / duration.Seconds() * 100
				if percent > 100 {
					percent = 100
				}
			}
			emitProgress(onProgress, Progress{
				Percent:       percent,
				SegmentText:   formatSegmentLine(ev.Start, ev.End, ev.Text),
				AudioDuration: duration,
			})
		case "error":
			runnerErr = ev.Message
		}
	}
	return segs, language, runnerErr, sc.Err()
}

// formatSegmentLine renders a segment the way the live preview already shows
// openai-whisper output, so both backends look the same in the TUI.
func formatSegmentLine(start, end float64, text string) string {
	return fmt.Sprintf("[%s --> %s] %s",
		formatTimestamp(start, '.'), formatTimestamp(end, '.'), strings.TrimSpace(text))
}
