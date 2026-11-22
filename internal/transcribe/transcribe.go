package transcribe

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"anoted/internal/config"
	"anoted/internal/recorder"
)

const (
	BackendAuto          = "auto"
	BackendOpenAI        = "openai-whisper"
	BackendWhisperCpp    = "whisper-cpp"
	BackendFasterWhisper = "faster-whisper"
	BackendParakeet      = "parakeet"
	DeviceCPU            = "cpu"
	DeviceCUDA           = "cuda"
	DeviceAuto           = "auto"
	TranscriptBaseName   = "transcript"
)

// Result holds generated transcript file paths.
type Result struct {
	SessionDir    string
	TranscriptDir string
	Files         []string
	// AudioDuration is filled for standalone files, where there is no session
	// metadata to carry it; zero when unknown.
	AudioDuration time.Duration
}

// Service transcribes session audio with Whisper.
type Service struct {
	cfg config.Config
}

func New(cfg config.Config) *Service {
	return &Service{cfg: cfg}
}

// TranscribeSession writes transcript.* files into sessionDir.
func (s *Service) TranscribeSession(ctx context.Context, sessionDir string) (Result, error) {
	return s.TranscribeSessionWithProgress(ctx, sessionDir, nil)
}

// TranscribeSessionWithProgress writes transcript.* files and reports progress via onProgress.
func (s *Service) TranscribeSessionWithProgress(ctx context.Context, sessionDir string, onProgress ProgressFunc) (Result, error) {
	audioPath := filepath.Join(sessionDir, recorder.SessionAudioFile)
	if _, err := os.Stat(audioPath); err != nil {
		return Result{}, fmt.Errorf("audio file missing: %w", err)
	}
	return s.transcribeAudio(ctx, s.cfg.Transcription, audioPath, sessionDir, onProgress)
}

// transcribeAudio runs the configured backend over audioPath and writes the
// outputs tcfg asks for. sessionDir names the outputs and is where they land
// unless tcfg.OutputDir says otherwise; the audio itself may live elsewhere,
// which is what lets a file from outside the library be transcribed.
func (s *Service) transcribeAudio(ctx context.Context, tcfg config.TranscriptionConfig, audioPath, sessionDir string, onProgress ProgressFunc) (Result, error) {
	outDir, err := OutputDir(tcfg, sessionDir)
	if err != nil {
		return Result{}, err
	}

	bin, backend, err := resolveBinary(tcfg)
	if err != nil {
		slog.Error("transcription backend unavailable", "err", err,
			"configured_backend", tcfg.Backend)
		return Result{}, err
	}

	// Transcription is the longest operation anoted performs and it was
	// completely silent in the log, so a stalled or mis-routed run left nothing
	// to diagnose — not even which engine actually ran.
	started := time.Now()
	slog.Info("transcription started",
		"backend", backend,
		"binary", bin,
		"model", resolvedModel(tcfg),
		"device", resolveDevice(tcfg),
		"session_dir", sessionDir,
		"output_dir", outDir,
	)
	defer func() {
		slog.Info("transcription finished",
			"backend", backend,
			"session_dir", sessionDir,
			"elapsed_ms", time.Since(started).Milliseconds(),
		)
	}()

	// Announce the engine in the live preview: which backend actually ran was
	// not visible anywhere in the UI, so a run could not be told apart from one
	// using a different engine than the Config tab displayed.
	emitProgress(onProgress, Progress{
		SegmentText: fmt.Sprintf("· %s · %s · %s",
			backend, resolvedModel(tcfg), resolveDevice(tcfg)),
	})

	err = transcribeGuardingVRAM(ctx, tcfg, onProgress, func(ctx context.Context) error {
		switch backend {
		case BackendFasterWhisper:
			// bin is the Python interpreter here, not a CLI: faster-whisper is a
			// library, so anoted renders the output files itself from its segments.
			_, err := transcribeFasterWhisper(ctx, tcfg, bin, audioPath, outDir, sessionDir, onProgress)
			return err
		case BackendParakeet:
			_, err := transcribeParakeet(ctx, tcfg, bin, audioPath, outDir, sessionDir, onProgress)
			return err
		case BackendWhisperCpp:
			_, err := transcribeWhisperCpp(ctx, tcfg, bin, audioPath, outDir, sessionDir, onProgress)
			return err
		default:
			_, err := transcribeOpenAI(ctx, tcfg, bin, audioPath, outDir, sessionDir, onProgress)
			return err
		}
	})
	if err != nil {
		return Result{}, err
	}
	res, err := postProcessTranscription(tcfg, sessionDir, outDir)
	if err != nil {
		return Result{}, err
	}
	res.SessionDir = sessionDir
	res.TranscriptDir = outDir
	return res, nil
}

// AudioPath returns the expected recording path inside a session directory.
func AudioPath(sessionDir string) string {
	return filepath.Join(sessionDir, recorder.SessionAudioFile)
}

// HasTranscript reports whether transcript output exists for a session.
func HasTranscript(sessionDir string, tcfg config.TranscriptionConfig) bool {
	dir, err := OutputDir(tcfg, sessionDir)
	if err != nil {
		dir = sessionDir
	}
	return hasTranscriptInDir(dir, tcfg, sessionDir)
}

func hasTranscriptInDir(dir string, tcfg config.TranscriptionConfig, sessionDir string) bool {
	return len(transcriptFilesInDir(dir, tcfg, sessionDir)) > 0
}

// AllTranscriptFiles returns every transcript file on disk for a session in
// any format, not only the ones output_formats asks for now. A session
// transcribed before the user changed formats still has its transcript.
func AllTranscriptFiles(sessionDir string, tcfg config.TranscriptionConfig) []string {
	dir, err := OutputDir(tcfg, sessionDir)
	if err != nil {
		dir = sessionDir
	}
	return transcriptFilesInDir(dir, tcfg, sessionDir)
}

func transcriptFilesInDir(dir string, tcfg config.TranscriptionConfig, sessionDir string) []string {
	fileBase := outputFileBase(tcfg, sessionDir)
	var out []string
	for _, ext := range []string{".txt", ".srt", ".vtt", ".json"} {
		p := filepath.Join(dir, fileBase+ext)
		if _, err := os.Stat(p); err == nil {
			out = append(out, p)
		}
	}
	md := filepath.Join(dir, markdownFilename(tcfg, sessionDir))
	if _, err := os.Stat(md); err == nil {
		out = append(out, md)
	}
	return out
}

// ListTranscriptFiles returns existing transcript output files for a session.
func ListTranscriptFiles(sessionDir string, tcfg config.TranscriptionConfig) []string {
	dir, err := OutputDir(tcfg, sessionDir)
	if err != nil {
		dir = sessionDir
	}
	return listOutputFiles(dir, tcfg, sessionDir)
}
