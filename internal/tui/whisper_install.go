package tui

import (
	"context"
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"anoted/internal/config"
	"anoted/internal/transcribe"
)

const whisperInstallLogMax = 40

type whisperInstallProgressMsg struct {
	line string
}

type whisperInstallResultMsg struct {
	err error
}

type whisperInstallEnvelopeMsg struct {
	ch    <-chan tea.Msg
	inner tea.Msg
}

func whisperInstallCmd(ctx context.Context) tea.Cmd {
	ch := make(chan tea.Msg, 32)

	go func() {
		send := func(line string) {
			select {
			case ch <- whisperInstallProgressMsg{line: line}:
			case <-ctx.Done():
			}
		}

		capture := &lineCapture{onLine: send}
		err := transcribe.EnsureWhisperCaptured(capture, true)
		capture.flush(send)
		sendWhisperInstallMsg(ch, ctx, whisperInstallResultMsg{err: err})
	}()

	return waitWhisperInstallMsg(ch)
}

type lineCapture struct {
	onLine func(string)
	buf    []byte
}

func (c *lineCapture) Write(p []byte) (int, error) {
	c.buf = append(c.buf, p...)
	for {
		i := bytesIndexByte(c.buf, '\n')
		if i < 0 {
			break
		}
		line := strings.TrimSpace(string(c.buf[:i]))
		c.buf = c.buf[i+1:]
		if line != "" && c.onLine != nil {
			c.onLine(line)
		}
	}
	return len(p), nil
}

func (c *lineCapture) flush(onLine func(string)) {
	if len(c.buf) == 0 {
		return
	}
	line := strings.TrimSpace(string(c.buf))
	c.buf = nil
	if line != "" {
		onLine(line)
	}
}

func bytesIndexByte(b []byte, c byte) int {
	for i, v := range b {
		if v == c {
			return i
		}
	}
	return -1
}

func sendWhisperInstallMsg(ch chan<- tea.Msg, ctx context.Context, msg tea.Msg) {
	select {
	case ch <- msg:
	case <-ctx.Done():
	}
	close(ch)
}

func waitWhisperInstallMsg(ch <-chan tea.Msg) tea.Cmd {
	return func() tea.Msg {
		msg, ok := <-ch
		if !ok {
			return whisperInstallResultMsg{err: context.Canceled}
		}
		return whisperInstallEnvelopeMsg{ch: ch, inner: msg}
	}
}

func (m Model) appendWhisperInstallLog(line string) {
	line = strings.TrimSpace(line)
	if line == "" {
		return
	}
	const max = 120
	if len(line) > max {
		line = line[:max-1] + "…"
	}
	m.whisperInstallLog = append(m.whisperInstallLog, line)
	if len(m.whisperInstallLog) > whisperInstallLogMax {
		m.whisperInstallLog = m.whisperInstallLog[len(m.whisperInstallLog)-whisperInstallLogMax:]
	}
}

func (m Model) startWhisperInstall() (Model, tea.Cmd) {
	if m.whisperInstallActive || transcribe.IsInstalled(m.deps.Config) {
		return m, nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	m.whisperInstallCancel = cancel
	m.whisperInstallActive = true
	m.whisperInstallLog = []string{"starting whisper install…"}
	m.whisperInstallErr = ""
	return m, whisperInstallCmd(ctx)
}

func (m Model) handleWhisperInstallEnvelope(msg whisperInstallEnvelopeMsg) (tea.Model, tea.Cmd) {
	switch inner := msg.inner.(type) {
	case whisperInstallProgressMsg:
		m.appendWhisperInstallLog(inner.line)
		return m, waitWhisperInstallMsg(msg.ch)
	case whisperInstallResultMsg:
		return m.handleWhisperInstallResult(inner)
	default:
		return m, waitWhisperInstallMsg(msg.ch)
	}
}

func (m Model) handleWhisperInstallResult(msg whisperInstallResultMsg) (tea.Model, tea.Cmd) {
	m.whisperInstallActive = false
	if m.whisperInstallCancel != nil {
		m.whisperInstallCancel = nil
	}

	if msg.err != nil {
		m.whisperInstallErr = msg.err.Error()
		m.appendWhisperInstallLog(fmt.Sprintf("failed: %v", msg.err))
		return m, nil
	}

	cfg := m.deps.Config
	cfg.Transcription.Binary = transcribe.ManagedWhisperBinary()
	cfg.Transcription.Backend = transcribe.BackendOpenAI
	cfg.Transcription.Device = transcribe.DeviceCPU
	cfg.Transcription.GPULayers = 0
	cfg.SetupCompleted = true

	path := m.deps.ConfigPath
	return m, func() tea.Msg {
		if err := config.Save(path, cfg); err != nil {
			return whisperInstallSavedMsg{err: err}
		}
		return whisperInstallSavedMsg{cfg: cfg}
	}
}

type whisperInstallSavedMsg struct {
	cfg config.Config
	err error
}

func (m Model) handleWhisperInstallSaved(msg whisperInstallSavedMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.whisperInstallErr = msg.err.Error()
		return m, nil
	}
	m.deps.Config = msg.cfg
	m.whisperInstallErr = ""
	m.appendWhisperInstallLog("✓ whisper installed")
	m.doctorReport = loadDoctorReport(m.deps.Config)
	return m, nil
}

func (m Model) whisperCanInstall() bool {
	return !m.whisperInstallActive && !transcribe.IsInstalled(m.deps.Config)
}
