package transcribe

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"meetctl/internal/config"
	"meetctl/internal/recorder"
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
	SessionDir string
	Files      []string
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
	audioPath := filepath.Join(sessionDir, recorder.SessionAudioFile)
	if _, err := os.Stat(audioPath); err != nil {
		return Result{}, fmt.Errorf("audio file missing: %w", err)
	}

	bin, backend, err := resolveBinary(s.cfg.Transcription)
	if err != nil {
		return Result{}, err
	}

	switch backend {
	case BackendWhisperCpp:
		return transcribeWhisperCpp(ctx, s.cfg.Transcription, bin, audioPath, sessionDir)
	default:
		return transcribeOpenAI(ctx, s.cfg.Transcription, bin, audioPath, sessionDir)
	}
}

// AudioPath returns the expected recording path inside a session directory.
func AudioPath(sessionDir string) string {
	return filepath.Join(sessionDir, recorder.SessionAudioFile)
}

// HasTranscript reports whether transcript.txt exists in the session folder.
func HasTranscript(sessionDir string) bool {
	_, err := os.Stat(filepath.Join(sessionDir, TranscriptBaseName+".txt"))
	return err == nil
}

// ListTranscriptFiles returns existing transcript.* files in the session folder.
func ListTranscriptFiles(sessionDir string) []string {
	var out []string
	for _, ext := range []string{".txt", ".srt", ".vtt", ".json"} {
		p := filepath.Join(sessionDir, TranscriptBaseName+ext)
		if _, err := os.Stat(p); err == nil {
			out = append(out, p)
		}
	}
	return out
}
