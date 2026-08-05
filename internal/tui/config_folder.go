package tui

import (
	"context"
	"os"
	"strings"

	"anoted/internal/config"
	"anoted/internal/folderpicker"
	tea "charm.land/bubbletea/v2"
)

type configFolderPickMsg struct {
	path     string
	canceled bool
	err      error
}

func (m Model) pickConfigFolderCmd() tea.Cmd {
	startDir := m.configFolderStartDir()
	return func() tea.Msg {
		path, canceled, err := folderpicker.Pick(context.Background(), startDir)
		return configFolderPickMsg{path: path, canceled: canceled, err: err}
	}
}

func (m Model) configFolderStartDir() string {
	f, ok := m.currentCfgField()
	if !ok {
		return ""
	}
	val := strings.TrimSpace(f.get(m.deps.Config))
	for _, placeholder := range []string{"(same as recording)", "(default)", "(auto-detect)", "(empty)"} {
		if val == placeholder {
			val = ""
			break
		}
	}
	if val != "" {
		if expanded, err := config.ExpandPath(val); err == nil {
			if st, err := os.Stat(expanded); err == nil && st.IsDir() {
				return expanded
			}
		}
	}
	if dir := strings.TrimSpace(m.deps.Config.OutputDir); dir != "" {
		if expanded, err := config.ExpandPath(dir); err == nil {
			if st, err := os.Stat(expanded); err == nil && st.IsDir() {
				return expanded
			}
		}
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return home
}

func (m Model) handleConfigFolderPick(msg configFolderPickMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.configErr = msg.err.Error()
		m.configSavedMsg = ""
		return m, nil
	}
	if msg.canceled || strings.TrimSpace(msg.path) == "" {
		return m, nil
	}
	m.configErr = ""
	return m, m.applyConfigValue(msg.path)
}

func (m Model) startConfigPathEdit() (tea.Model, tea.Cmd) {
	f, ok := m.currentCfgField()
	if !ok || f.kind != fieldPath {
		return m, nil
	}
	if f.editable != nil && !f.editable(m.deps.Config) {
		return m, nil
	}
	m.configEditing = true
	val := f.get(m.deps.Config)
	if val == "(same as recording)" || val == "(default)" || val == "(auto-detect)" || val == "(empty)" {
		m.configInput = ""
	} else {
		m.configInput = val
	}
	return m, nil
}

func (m Model) clearConfigPathField() (tea.Model, tea.Cmd) {
	f, ok := m.currentCfgField()
	if !ok || f.kind != fieldPath {
		return m, nil
	}
	return m, m.applyConfigValue("")
}
