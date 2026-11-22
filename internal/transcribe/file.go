package transcribe

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"anoted/internal/config"
	"anoted/internal/recorder"
)

// fileTranscriptionConfig is the configuration a standalone file is
// transcribed with: the same engine, model, language and device as meetings,
// but a single plain-text transcript kept next to the job.
//
// The output overrides are the point. Meetings are commonly configured to land
// as Markdown notes in a vault (transcription.output_dir plus the md format),
// and a voice note dragged in to be read once has no business there — nor does
// it have the session metadata that note's frontmatter is built from.
func fileTranscriptionConfig(tcfg config.TranscriptionConfig) config.TranscriptionConfig {
	tcfg.OutputDir = ""
	tcfg.OutputFormats = []string{config.OutputFormatTXT}
	return tcfg
}

// TranscribeFileWithProgress transcribes an audio or video file from anywhere
// on disk and writes transcript.txt into workDir. The source is only read.
//
// When ffmpeg is available the file is first decoded to 16 kHz mono PCM. That
// is what makes every backend accept every container — whisper.cpp reads WAV
// only — picks the audio track out of a video, and gives the progress bar a
// duration to measure against. Without ffmpeg the source goes to the backend
// as it is, which still covers WAV everywhere and anything faster-whisper can
// decode on its own.
func (s *Service) TranscribeFileWithProgress(ctx context.Context, source, workDir string, onProgress ProgressFunc) (Result, error) {
	st, err := os.Stat(source)
	if err != nil {
		return Result{}, fmt.Errorf("source file missing: %w", err)
	}
	if st.IsDir() {
		return Result{}, fmt.Errorf("%s is a folder, not an audio file", source)
	}
	if err := os.MkdirAll(workDir, recorder.SessionDirMode); err != nil {
		return Result{}, fmt.Errorf("create %s: %w", workDir, err)
	}

	audioPath := source
	if ffmpeg, lookErr := exec.LookPath("ffmpeg"); lookErr == nil {
		emitProgress(onProgress, Progress{SegmentText: "· decoding audio with ffmpeg…"})
		wav := filepath.Join(workDir, recorder.SessionAudioFile)
		// The decoded copy is scratch: the source stays where the user keeps it,
		// so holding on to a second copy would only grow the data directory.
		defer os.Remove(wav)
		if err := decodeToWAV(ctx, ffmpeg, source, wav); err != nil {
			return Result{}, err
		}
		audioPath = wav
	}

	duration, _ := AudioDuration(audioPath)
	res, err := s.transcribeAudio(ctx, fileTranscriptionConfig(s.cfg.Transcription), audioPath, workDir, onProgress)
	res.AudioDuration = duration
	return res, err
}

// decodeToWAV writes the first audio stream of src as 16 kHz mono 16-bit PCM,
// the one format every backend reads without help.
func decodeToWAV(ctx context.Context, ffmpeg, src, dst string) error {
	cmd := exec.CommandContext(ctx, ffmpeg,
		"-nostdin", "-v", "error", "-y",
		"-i", src,
		"-map", "0:a:0", "-vn", "-sn", "-dn",
		"-ac", "1", "-ar", "16000", "-c:a", "pcm_s16le",
		dst,
	)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		// CommandContext kills the child on cancellation and Run then reports
		// the signal, which would surface a user's stop as a red error.
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("ffmpeg could not read %s: %s", filepath.Base(src), ffmpegReason(stderr.String(), err))
	}
	return nil
}

// ffmpegReason reduces ffmpeg's stderr to the line that says what went wrong.
func ffmpegReason(stderr string, err error) string {
	if strings.Contains(stderr, "matches no streams") {
		return "it has no audio track"
	}
	lines := strings.Split(strings.TrimSpace(stderr), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		if line := strings.TrimSpace(lines[i]); line != "" {
			// ffmpeg prefixes most errors with the path it was given.
			if idx := strings.LastIndex(line, ": "); idx >= 0 && idx+2 < len(line) {
				line = line[idx+2:]
			}
			return line
		}
	}
	return err.Error()
}
