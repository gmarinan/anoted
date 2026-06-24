package transcribe

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"anoted/internal/config"
	"anoted/internal/recorder"
)

const (
	BackendAuto        = "auto"
	BackendOpenAI      = "openai-whisper"
	BackendWhisperCpp  = "whisper-cpp"
	DeviceCPU          = "cpu"
	DeviceCUDA         = "cuda"
	DeviceAuto         = "auto"
	TranscriptBaseName = "transcript"
)

// Result holds generated transcript file paths.
type Result struct {
	SessionDir   string
	TranscriptDir string
	Files        []string
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

	outDir, err := OutputDir(s.cfg.Transcription, sessionDir)
	if err != nil {
		return Result{}, err
	}

	bin, backend, err := resolveBinary(s.cfg.Transcription)
	if err != nil {
		return Result{}, err
	}

	switch backend {
	case BackendWhisperCpp:
		if _, err := transcribeWhisperCpp(ctx, s.cfg.Transcription, bin, audioPath, outDir, sessionDir, onProgress); err != nil {
			return Result{}, err
		}
	default:
		if _, err := transcribeOpenAI(ctx, s.cfg.Transcription, bin, audioPath, outDir, sessionDir, onProgress); err != nil {
			return Result{}, err
		}
	}
	res, err := postProcessTranscription(s.cfg.Transcription, sessionDir, outDir)
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
	fileBase := outputFileBase(tcfg, sessionDir)
	for _, ext := range []string{".txt", ".srt", ".vtt", ".json"} {
		p := filepath.Join(dir, fileBase+ext)
		if _, err := os.Stat(p); err == nil {
			return true
		}
	}
	md := filepath.Join(dir, markdownFilename(tcfg, sessionDir))
	if _, err := os.Stat(md); err == nil {
		return true
	}
	return false
}

// ListTranscriptFiles returns existing transcript output files for a session.
func ListTranscriptFiles(sessionDir string, tcfg config.TranscriptionConfig) []string {
	dir, err := OutputDir(tcfg, sessionDir)
	if err != nil {
		dir = sessionDir
	}
	return listOutputFiles(dir, tcfg, sessionDir)
}
