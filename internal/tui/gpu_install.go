package tui

import (
	"context"
	"fmt"
	"strings"

	"anoted/internal/config"
	"anoted/internal/setup"
	"anoted/internal/transcribe"
	tea "charm.land/bubbletea/v2"
)

const gpuInstallLogMax = 40

type gpuInstallProgressMsg struct {
	line string
}

type gpuInstallResultMsg struct {
	err error
}

type gpuInstallEnvelopeMsg struct {
	ch    <-chan tea.Msg
	inner tea.Msg
}

func gpuInstallCmd(ctx context.Context) tea.Cmd {
	ch := make(chan tea.Msg, 32)

	go func() {
		send := func(line string) {
			select {
			case ch <- gpuInstallProgressMsg{line: line}:
			case <-ctx.Done():
			}
		}

		capture := &lineCapture{onLine: send}
		err := transcribe.UpgradeManagedTorchCUDACaptured(capture, capture, capture)
		capture.flush(send)
		sendGPUInstallMsg(ch, ctx, gpuInstallResultMsg{err: err})
	}()

	return waitGPUInstallMsg(ch)
}

func sendGPUInstallMsg(ch chan<- tea.Msg, ctx context.Context, msg tea.Msg) {
	select {
	case ch <- msg:
	case <-ctx.Done():
	}
	close(ch)
}

func waitGPUInstallMsg(ch <-chan tea.Msg) tea.Cmd {
	return func() tea.Msg {
		msg, ok := <-ch
		if !ok {
			return gpuInstallResultMsg{err: context.Canceled}
		}
		return gpuInstallEnvelopeMsg{ch: ch, inner: msg}
	}
}

// Pointer receiver: see appendTranscribeLog.
func (m *Model) appendGPUInstallLog(line string) {
	line = strings.TrimSpace(line)
	if line == "" {
		return
	}
	const max = 120
	if len(line) > max {
		line = line[:max-1] + "…"
	}
	m.gpuInstallLog = append(m.gpuInstallLog, line)
	if len(m.gpuInstallLog) > gpuInstallLogMax {
		m.gpuInstallLog = m.gpuInstallLog[len(m.gpuInstallLog)-gpuInstallLogMax:]
	}
}

func (m Model) startGPUInstall() (Model, tea.Cmd) {
	if m.gpuInstallActive {
		return m, nil
	}
	if !setup.GPUOfferAvailable(m.deps.Config) {
		return m, nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	m.gpuInstallCancel = cancel
	m.gpuInstallActive = true
	m.gpuInstallLog = []string{"enabling GPU (PyTorch CUDA)…"}
	m.gpuInstallErr = ""
	m.gpuInstallScroll = 0
	return m, gpuInstallCmd(ctx)
}

func (m Model) handleGPUInstallEnvelope(msg gpuInstallEnvelopeMsg) (tea.Model, tea.Cmd) {
	switch inner := msg.inner.(type) {
	case gpuInstallProgressMsg:
		m.appendGPUInstallLog(inner.line)
		m.gpuInstallScroll = m.maxGPUInstallScroll(8)
		return m, waitGPUInstallMsg(msg.ch)
	case gpuInstallResultMsg:
		return m.handleGPUInstallResult(inner)
	default:
		return m, waitGPUInstallMsg(msg.ch)
	}
}

func (m Model) handleGPUInstallResult(msg gpuInstallResultMsg) (tea.Model, tea.Cmd) {
	m.gpuInstallActive = false
	if m.gpuInstallCancel != nil {
		m.gpuInstallCancel = nil
	}

	if msg.err != nil {
		m.gpuInstallErr = msg.err.Error()
		m.appendGPUInstallLog(fmt.Sprintf("failed: %v", msg.err))
		return m, nil
	}

	cfg := m.deps.Config
	cfg.Transcription.Device = transcribe.DeviceCUDA
	cfg.Transcription.GPULayers = 0

	path := m.deps.ConfigPath
	return m, func() tea.Msg {
		if err := config.Save(path, cfg); err != nil {
			return gpuInstallSavedMsg{err: err}
		}
		return gpuInstallSavedMsg{cfg: cfg}
	}
}

type gpuInstallSavedMsg struct {
	cfg config.Config
	err error
}

func (m Model) handleGPUInstallSaved(msg gpuInstallSavedMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.gpuInstallErr = msg.err.Error()
		return m, nil
	}
	m.deps.Config = msg.cfg
	m.deps.Transcriber = transcribe.New(msg.cfg)
	m.gpuInstallErr = ""
	m.appendGPUInstallLog("✓ GPU enabled (PyTorch CUDA)")
	transcribe.InvalidateTorchCUDACache()
	m.doctorGPUCanInstall = false
	return m, doctorReportCmd(m.deps.Config)
}

func (m Model) maxGPUInstallScroll(viewHeight int) int {
	n := len(m.gpuInstallLog) - viewHeight
	if n < 0 {
		return 0
	}
	return n
}

func (m Model) visibleGPUInstallLog(viewHeight int) []string {
	if viewHeight < 1 {
		viewHeight = 6
	}
	if len(m.gpuInstallLog) <= viewHeight {
		return m.gpuInstallLog
	}
	start := m.gpuInstallScroll
	maxStart := m.maxGPUInstallScroll(viewHeight)
	if start > maxStart {
		start = maxStart
	}
	if start < 0 {
		start = 0
	}
	return m.gpuInstallLog[start : start+viewHeight]
}
