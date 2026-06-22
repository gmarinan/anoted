package recorder

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"anoted/internal/session"
)

func TestDummyRecorderStartStop(t *testing.T) {
	r := NewDummyRecorder()
	root := t.TempDir()
	err := r.Start(context.Background(), SessionConfig{
		OutputRoot: root,
		Provider:   session.ProviderTeams,
		Platform:   "test",
		Manual:     true,
		SampleRate: 48000,
		Channels:   2,
	})
	if err != nil {
		t.Fatal(err)
	}
	st := r.Status()
	if st.Status != StatusRecording {
		t.Fatalf("expected recording, got %s", st.Status)
	}
	if _, err := os.Stat(filepath.Join(st.SessionDir, SessionAudioFile)); err != nil {
		t.Fatalf("expected %s: %v", SessionAudioFile, err)
	}
	if err := r.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
}
