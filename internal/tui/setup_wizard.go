package tui

import (
	"context"
	"fmt"

	"anoted/internal/config"
	"anoted/internal/detector"
	"anoted/internal/setup"
	"anoted/internal/transcribe"
	"anoted/internal/tui/components"
	tea "charm.land/bubbletea/v2"
)

type setupInstallProgressMsg struct {
	line string
}

type setupInstallResultMsg struct {
	cfg config.Config
	err error
}

type setupInstallEnvelopeMsg struct {
	ch    <-chan tea.Msg
	inner tea.Msg
}

func (m Model) openSetupWizard() Model {
	m.setupOpen = true
	m.setupWizard = setup.NewWizardState(m.deps.Platform)
	m.setupSummary = nil
	return m
}

func (m Model) closeSetupWizard() Model {
	m.setupOpen = false
	m.setupWizard.Busy = false
	if m.setupCancel != nil {
		m.setupCancel()
		m.setupCancel = nil
	}
	return m
}

func (m Model) setupAbsorbsKeys() bool {
	return m.setupOpen
}

func (m Model) handleSetupKey(key string) (tea.Model, tea.Cmd) {
	if m.setupWizard.Busy {
		switch key {
		case "pgup":
			m.setupWizard.LogScroll -= 3
			return m, nil
		case "pgdown":
			m.setupWizard.LogScroll += 3
			return m, nil
		}
		return m, nil
	}

	switch key {
	case "esc":
		switch m.setupWizard.Step {
		case setup.WizardWelcome:
			return m.closeSetupWizard(), nil
		case setup.WizardDetection:
			m.setupWizard.Step = setup.WizardWelcome
			return m, nil
		case setup.WizardTranscription:
			m.setupWizard.Step = setup.WizardDetection
			return m, nil
		case setup.WizardInstalling:
			if m.setupWizard.Err != "" {
				m.setupWizard.Step = setup.WizardTranscription
				m.setupWizard.Err = ""
			}
			return m, nil
		default:
			return m, nil
		}
	case "enter":
		if m.setupWizard.Step == setup.WizardInstalling && m.setupWizard.Err != "" {
			m.setupWizard.Step = setup.WizardTranscription
			m.setupWizard.Err = ""
			return m, nil
		}
		return m.setupAdvance()
	case "up", "k":
		return m.setupNavUp(), nil
	case "down", "j":
		return m.setupNavDown(), nil
	case " ":
		if m.setupWizard.Step == setup.WizardTranscription {
			switch m.setupWizard.TranscribeCursor {
			case 0:
				m.setupWizard.AutoTranscribe = !m.setupWizard.AutoTranscribe
			case 1:
				m.setupWizard.InstallWhisper = !m.setupWizard.InstallWhisper
			case 2:
				m.setupWizard.EnableGPU = !m.setupWizard.EnableGPU
			}
		}
		return m, nil
	case "pgup":
		m.setupWizard.LogScroll -= 3
		return m, nil
	case "pgdown":
		m.setupWizard.LogScroll += 3
		return m, nil
	}
	return m, nil
}

func (m Model) setupNavUp() Model {
	switch m.setupWizard.Step {
	case setup.WizardDetection:
		if m.setupWizard.DetCursor > 0 {
			m.setupWizard.DetCursor--
		}
	case setup.WizardTranscription:
		if m.setupWizard.TranscribeCursor > 0 {
			m.setupWizard.TranscribeCursor--
		}
	}
	return m
}

func (m Model) setupNavDown() Model {
	switch m.setupWizard.Step {
	case setup.WizardDetection:
		choices := setup.DetectionChoices(m.deps.Platform)
		if m.setupWizard.DetCursor < len(choices)-1 {
			m.setupWizard.DetCursor++
		}
	case setup.WizardTranscription:
		max := setup.TranscribeOptionCount(m.deps.Config) - 1
		if m.setupWizard.TranscribeCursor < max {
			m.setupWizard.TranscribeCursor++
		}
	}
	return m
}

func (m Model) setupAdvance() (tea.Model, tea.Cmd) {
	switch m.setupWizard.Step {
	case setup.WizardWelcome:
		m.setupWizard.Step = setup.WizardDetection
		return m, nil
	case setup.WizardDetection:
		mode := m.setupWizard.SelectedDetectionMode(m.deps.Platform)
		cfg := m.deps.Config
		cfg.Detection.Mode = mode
		m.setupWizard.DetectionLines = setup.ApplyDetection(&cfg, m.deps.Platform, mode)
		m = m.applyConfig(cfg)
		m.setupWizard.Step = setup.WizardTranscription
		return m, nil
	case setup.WizardTranscription:
		m.setupWizard.Step = setup.WizardInstalling
		m.setupWizard.Busy = true
		m.setupWizard.Err = ""
		m.setupWizard.Log = []string{"starting setup…"}
		m.setupWizard.LogScroll = 0
		ctx, cancel := context.WithCancel(context.Background())
		m.setupCancel = cancel
		return m, tea.Batch(m.runSetupInstallCmd(ctx), m.scheduleInstallSpin())
	case setup.WizardDone:
		return m.closeSetupWizard(), nil
	}
	return m, nil
}

func (m Model) runSetupInstallCmd(ctx context.Context) tea.Cmd {
	ch := make(chan tea.Msg, 64)
	cfg := m.deps.Config
	path := m.deps.ConfigPath
	plat := m.deps.Platform
	w := m.setupWizard

	go func() {
		send := func(line string) {
			select {
			case ch <- setupInstallProgressMsg{line: line}:
			case <-ctx.Done():
			}
		}
		capture := &lineCapture{onLine: send}

		mode := w.SelectedDetectionMode(plat)
		cfg.Detection.Mode = mode
		for _, line := range setup.ApplyDetection(&cfg, plat, mode) {
			send(line)
		}

		plan := setup.TranscriptionPlan{
			AutoAfterRecording: w.AutoTranscribe,
			InstallWhisper:     w.InstallWhisper,
			EnableGPU:          w.EnableGPU,
		}
		installErr := setup.ConfigureTranscription(&cfg, plan, nil, capture, true)
		capture.flush(send)

		cfg.SetupCompleted = true
		if saveErr := config.Save(path, cfg); saveErr != nil && installErr == nil {
			installErr = saveErr
		}
		if installErr != nil {
			send(fmt.Sprintf("error: %v", installErr))
			_ = config.Save(path, cfg)
			sendSetupInstallMsg(ch, ctx, setupInstallResultMsg{cfg: cfg, err: installErr})
			return
		}
		send("✓ setup saved")
		sendSetupInstallMsg(ch, ctx, setupInstallResultMsg{cfg: cfg})
	}()

	return waitSetupInstallMsg(ch)
}

func sendSetupInstallMsg(ch chan<- tea.Msg, ctx context.Context, msg tea.Msg) {
	select {
	case ch <- msg:
	case <-ctx.Done():
	}
	close(ch)
}

func waitSetupInstallMsg(ch <-chan tea.Msg) tea.Cmd {
	return func() tea.Msg {
		msg, ok := <-ch
		if !ok {
			return setupInstallResultMsg{err: context.Canceled}
		}
		return setupInstallEnvelopeMsg{ch: ch, inner: msg}
	}
}

func (m Model) handleSetupInstallEnvelope(msg setupInstallEnvelopeMsg) (tea.Model, tea.Cmd) {
	switch inner := msg.inner.(type) {
	case setupInstallProgressMsg:
		m.setupWizard.AppendLog(inner.line)
		m.setupWizard.LogScroll = m.setupWizard.MaxLogScroll(10)
		return m, waitSetupInstallMsg(msg.ch)
	case setupInstallResultMsg:
		return m.handleSetupInstallResult(inner)
	default:
		return m, waitSetupInstallMsg(msg.ch)
	}
}

func (m Model) handleSetupInstallResult(msg setupInstallResultMsg) (tea.Model, tea.Cmd) {
	m.setupWizard.Busy = false
	if m.setupCancel != nil {
		m.setupCancel = nil
	}
	m = m.applyConfig(msg.cfg)
	m.deps.Detector = detector.New(msg.cfg, m.deps.Platform, false)
	m.deps.Transcriber = transcribe.New(msg.cfg)
	m.autoRecord = msg.cfg.AutoRecord
	m.setupSummary = setup.SummaryLines(msg.cfg)

	if msg.err != nil {
		m.setupWizard.Err = msg.err.Error()
		m.setupWizard.AppendLog("failed: " + msg.err.Error())
		m.setupWizard.Step = setup.WizardInstalling
		return m, tea.Batch(doctorReportCmd(m.deps.Config), refreshDoctorCapsCmd(m.deps.Config))
	}
	m.setupWizard.Err = ""
	m.setupWizard.Step = setup.WizardDone
	return m, tea.Batch(doctorReportCmd(m.deps.Config), refreshDoctorCapsCmd(m.deps.Config))
}

func (m Model) setupWizardOverlay(base string) string {
	if !m.setupOpen {
		return base
	}
	return components.SetupWizardView{
		State:        m.setupWizard,
		Choices:      setup.DetectionChoices(m.deps.Platform),
		Config:       m.deps.Config,
		ConfigPath:   m.deps.ConfigPath,
		Platform:     m.deps.Platform.Name(),
		Width:        m.width,
		Height:       m.height,
		Summary:      m.setupSummary,
		InstallFrame: m.installFrame,
	}.View(base)
}
