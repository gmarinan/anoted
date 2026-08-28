package tui

import "testing"

// These appenders had value receivers, so every line was written to a discarded
// copy: the transcription preview and both install panes stayed frozen on their
// seed line for the entire run.
func TestLogAppendersMutateModel(t *testing.T) {
	var m Model

	m.appendTranscribeLog("hello")
	if len(m.transcribeLog) != 1 || m.transcribeLog[0] != "hello" {
		t.Fatalf("appendTranscribeLog did not mutate: %v", m.transcribeLog)
	}

	m.whisperInstall.appendLine("installing")
	if len(m.whisperInstall.Log) != 1 {
		t.Fatalf("appendWhisperInstallLog did not mutate: %v", m.whisperInstall.Log)
	}

	m.gpuInstall.appendLine("cuda")
	if len(m.gpuInstall.Log) != 1 {
		t.Fatalf("appendGPUInstallLog did not mutate: %v", m.gpuInstall.Log)
	}

	// Blank lines are still ignored.
	m.appendTranscribeLog("   ")
	if len(m.transcribeLog) != 1 {
		t.Fatalf("blank line was appended: %v", m.transcribeLog)
	}
}
