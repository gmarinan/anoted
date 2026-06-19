package transcribe

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"meetctl/internal/config"
)

func transcribeOpenAI(ctx context.Context, cfg config.TranscriptionConfig, bin, audioPath, sessionDir string) (Result, error) {
	model := resolvedModel(cfg)
	args := []string{
		audioPath,
		"--model", model,
		"--output_dir", sessionDir,
		"--output_format", "txt,srt,vtt",
		"--verbose", "False",
	}
	if lang := strings.TrimSpace(cfg.Language); lang != "" {
		args = append(args, "--language", lang)
	}
	if resolveDevice(cfg) == DeviceCUDA {
		args = append(args, "--device", "cuda")
	} else {
		args = append(args, "--device", "cpu")
	}

	cmd := exec.CommandContext(ctx, bin, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return Result{}, fmt.Errorf("whisper: %w: %s", err, trimOutput(out))
	}

	base := strings.TrimSuffix(filepath.Base(audioPath), filepath.Ext(audioPath))
	renames := map[string]string{
		base + ".txt":  TranscriptBaseName + ".txt",
		base + ".srt":  TranscriptBaseName + ".srt",
		base + ".vtt":  TranscriptBaseName + ".vtt",
		base + ".json": TranscriptBaseName + ".json",
	}
	var files []string
	for src, dst := range renames {
		oldPath := filepath.Join(sessionDir, src)
		newPath := filepath.Join(sessionDir, dst)
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
	return Result{SessionDir: sessionDir, Files: files}, nil
}

func transcribeWhisperCpp(ctx context.Context, cfg config.TranscriptionConfig, bin, audioPath, sessionDir string) (Result, error) {
	modelPath, err := resolveCppModelPath(cfg)
	if err != nil {
		return Result{}, err
	}
	outPrefix := filepath.Join(sessionDir, TranscriptBaseName)
	args := []string{
		"-m", modelPath,
		"-f", audioPath,
		"-of", outPrefix,
		"-otxt", "-osrt",
	}
	if lang := strings.TrimSpace(cfg.Language); lang != "" {
		args = append(args, "-l", lang)
	}
	if cfg.GPULayers > 0 {
		args = append(args, "-ngl", fmt.Sprintf("%d", cfg.GPULayers))
	} else if resolveDevice(cfg) == DeviceCUDA {
		args = append(args, "-ngl", "99")
	}

	cmd := exec.CommandContext(ctx, bin, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return Result{}, fmt.Errorf("whisper.cpp: %w: %s", err, trimOutput(out))
	}

	files := ListTranscriptFiles(sessionDir)
	if len(files) == 0 {
		return Result{}, fmt.Errorf("whisper.cpp produced no output: %s", trimOutput(out))
	}
	return Result{SessionDir: sessionDir, Files: files}, nil
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
	s := strings.TrimSpace(string(b))
	if len(s) > 400 {
		return s[:400] + "…"
	}
	return s
}
