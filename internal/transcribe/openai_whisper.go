package transcribe

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"anoted/internal/config"
)

func transcribeOpenAI(ctx context.Context, cfg config.TranscriptionConfig, bin, audioPath, outDir, sessionDir string, onProgress ProgressFunc) (Result, error) {
	device := resolveDevice(cfg)
	out, err := runOpenAIWhisper(ctx, cfg, bin, audioPath, outDir, device, onProgress)
	if err != nil && device == DeviceCUDA && isCUDAFailure(out) {
		emitProgress(onProgress, Progress{
			SegmentText: "GPU out of memory — retrying on CPU…",
		})
		out, err = runOpenAIWhisper(ctx, cfg, bin, audioPath, outDir, DeviceCPU, onProgress)
	}
	if err != nil {
		return Result{}, formatWhisperError(ctx, "whisper", out, err)
	}
	fileBase := outputFileBase(cfg, sessionDir)
	return finalizeOpenAIResult(outDir, audioPath, fileBase)
}

func runOpenAIWhisper(ctx context.Context, cfg config.TranscriptionConfig, bin, audioPath, outDir, device string, onProgress ProgressFunc) ([]byte, error) {
	audioDur, _ := AudioDuration(audioPath)
	model := resolvedModel(cfg)
	formats := config.NormalizeOutputFormats(cfg.OutputFormats)
	args := []string{
		audioPath,
		"--model", model,
		"--output_dir", outDir,
		"--output_format", whisperOutputFormatArg(formats),
		"--verbose", "True",
		"--device", device,
	}
	if lang := strings.TrimSpace(cfg.Language); lang != "" {
		args = append(args, "--language", lang)
	}

	return runWithProgress(ctx, bin, args, onProgress, func(stream string, line string) {
		switch stream {
		case "stdout":
			if pct, seg, ok := ParseSegmentLine(line, audioDur); ok {
				emitProgress(onProgress, Progress{
					Percent:       pct,
					SegmentText:   seg,
					AudioDuration: audioDur,
				})
			}
		case "stderr":
			if pct, ok := ParseTqdmProgressLine(line); ok {
				emitProgress(onProgress, Progress{
					Percent:       pct,
					AudioDuration: audioDur,
				})
			} else if pct, seg, ok := ParseSegmentLine(line, audioDur); ok {
				emitProgress(onProgress, Progress{
					Percent:       pct,
					SegmentText:   seg,
					AudioDuration: audioDur,
				})
			}
		}
	})
}

func transcribeWhisperCpp(ctx context.Context, cfg config.TranscriptionConfig, bin, audioPath, outDir, sessionDir string, onProgress ProgressFunc) (Result, error) {
	audioDur, _ := AudioDuration(audioPath)
	modelPath, err := resolveCppModelPath(cfg)
	if err != nil {
		return Result{}, err
	}
	fileBase := outputFileBase(cfg, sessionDir)
	outPrefix := filepath.Join(outDir, fileBase)
	args := []string{
		"-m", modelPath,
		"-f", audioPath,
		"-of", outPrefix,
		"-pp",
	}
	args = appendCppOutputFlags(args, config.NormalizeOutputFormats(cfg.OutputFormats))
	if lang := strings.TrimSpace(cfg.Language); lang != "" {
		args = append(args, "-l", lang)
	}
	if cfg.GPULayers > 0 {
		args = append(args, "-ngl", fmt.Sprintf("%d", cfg.GPULayers))
	} else if resolveDevice(cfg) == DeviceCUDA {
		args = append(args, "-ngl", "99")
	}

	out, err := runWithProgress(ctx, bin, args, onProgress, func(stream string, line string) {
		switch stream {
		case "stderr":
			if pct, ok := ParseCppProgressLine(line); ok {
				emitProgress(onProgress, Progress{
					Percent:       pct,
					AudioDuration: audioDur,
				})
			}
		case "stdout":
			line = strings.TrimSpace(line)
			if line != "" {
				emitProgress(onProgress, Progress{
					SegmentText:   line,
					AudioDuration: audioDur,
				})
			}
		}
	})
	if err != nil {
		return Result{}, formatWhisperError(ctx, "whisper.cpp", out, err)
	}

	files := listOutputFiles(outDir, cfg, sessionDir)
	if len(files) == 0 {
		for _, ext := range []string{".txt", ".srt", ".vtt", ".json"} {
			p := filepath.Join(outDir, fileBase+ext)
			if _, statErr := os.Stat(p); statErr == nil {
				files = append(files, p)
			}
		}
	}
	if len(files) == 0 {
		return Result{}, fmt.Errorf("whisper.cpp produced no output: %s", trimOutput(out))
	}
	return Result{SessionDir: outDir, Files: files}, nil
}

func finalizeOpenAIResult(outDir, audioPath, fileBase string) (Result, error) {
	base := strings.TrimSuffix(filepath.Base(audioPath), filepath.Ext(audioPath))
	renames := map[string]string{
		base + ".txt":  fileBase + ".txt",
		base + ".srt":  fileBase + ".srt",
		base + ".vtt":  fileBase + ".vtt",
		base + ".json": fileBase + ".json",
	}
	var files []string
	for src, dst := range renames {
		oldPath := filepath.Join(outDir, src)
		newPath := filepath.Join(outDir, dst)
		if _, err := os.Stat(oldPath); err != nil {
			continue
		}
		if oldPath != newPath {
			_ = os.Remove(newPath)
			if err := os.Rename(oldPath, newPath); err != nil {
				return Result{}, fmt.Errorf("rename %s: %w", src, err)
			}
		}
		files = append(files, newPath)
	}
	if len(files) == 0 {
		return Result{}, fmt.Errorf("whisper produced no output files")
	}
	return Result{SessionDir: outDir, Files: files}, nil
}

func runWithProgress(ctx context.Context, bin string, args []string, onProgress ProgressFunc, handleLine func(stream, line string)) ([]byte, error) {
	cmd := exec.CommandContext(ctx, bin, args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, err
	}

	// Each scanner gets its own buffer: sharing one strings.Builder across the
	// two goroutines was a data race that dropped lines, and the merged output
	// is the only input to isCUDAFailure — losing the "CUDA out of memory" line
	// defeats the automatic retry on CPU.
	var outBuf, errBuf strings.Builder
	var scanErr [2]error
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		scanErr[0] = scanLines(stdout, func(line string) {
			outBuf.WriteString(line)
			outBuf.WriteByte('\n')
			if handleLine != nil {
				handleLine("stdout", line)
			}
		})
	}()
	go func() {
		defer wg.Done()
		scanErr[1] = scanLines(stderr, func(line string) {
			errBuf.WriteString(line)
			errBuf.WriteByte('\n')
			if handleLine != nil {
				handleLine("stderr", line)
			}
		})
	}()

	if err := cmd.Start(); err != nil {
		return nil, err
	}
	wg.Wait()
	err = cmd.Wait()

	out := []byte(outBuf.String() + errBuf.String())
	if err == nil {
		// A scanner that stops early leaves the pipe undrained, which is worth
		// knowing about — but it is not a transcription failure. whisper exiting
		// zero means the transcript is already on disk, and turning that into an
		// error discarded a finished run and showed the user a red row for a
		// session that had actually succeeded.
		for _, se := range scanErr {
			if se != nil {
				slog.Warn("whisper output truncated while reading", "err", se)
			}
		}
	}
	return out, err
}

func resolveCppModelPath(cfg config.TranscriptionConfig) (string, error) {
	if cfg.ModelPath != "" {
		if _, err := os.Stat(cfg.ModelPath); err != nil {
			return "", fmt.Errorf("model_path %q: %w", cfg.ModelPath, err)
		}
		return cfg.ModelPath, nil
	}
	model := resolvedModel(cfg)
	candidates := []string{
		filepath.Join(os.Getenv("HOME"), ".cache", "whisper", "ggml-"+model+".bin"),
		filepath.Join("/usr/share", "whisper.cpp", "ggml-"+model+".bin"),
		filepath.Join("/usr/share", "whisper", "ggml-"+model+".bin"),
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c, nil
		}
	}
	return "", fmt.Errorf("ggml model not found for %q — set transcription.model_path", model)
}

func trimOutput(b []byte) string {
	s := extractWhisperError(string(b))
	if s == "" {
		s = strings.TrimSpace(string(b))
	}
	if len(s) > 400 {
		return s[:400] + "…"
	}
	return s
}
