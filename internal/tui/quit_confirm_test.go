package tui

import "testing"

func TestQuitGuarded(t *testing.T) {
	m := Model{recording: true}
	if !m.quitGuarded() {
		t.Fatal("expected guarded while recording")
	}
	m.recording = false
	m.transcribeActive = true
	if !m.quitGuarded() {
		t.Fatal("expected guarded while transcribing")
	}
	m.transcribeActive = false
	m.whisperInstallActive = true
	if !m.quitGuarded() {
		t.Fatal("expected guarded while whisper installing")
	}
	m.whisperInstallActive = false
	m.gpuInstallActive = true
	if !m.quitGuarded() {
		t.Fatal("expected guarded while GPU installing")
	}
	m.gpuInstallActive = false
	if m.quitGuarded() {
		t.Fatal("expected not guarded when idle")
	}
}

func TestQuitConfirmReasons(t *testing.T) {
	m := Model{recording: true, transcribeActive: true}
	reasons := m.quitConfirmReasons()
	if len(reasons) != 2 {
		t.Fatalf("expected 2 reasons, got %d", len(reasons))
	}
}
