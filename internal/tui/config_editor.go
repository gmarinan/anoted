package tui

import (
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	"meetctl/internal/config"
	"meetctl/internal/tui/components"
)

func (m Model) loadConfigEditor() Model {
	raw, err := config.ReadRaw(m.deps.ConfigPath)
	if err != nil {
		m.configErr = err.Error()
		m.configLines = []string{""}
	} else {
		m.configErr = ""
		m.configLines = components.TextToLines(raw)
	}
	m.configCursorRow = 0
	m.configCursorCol = 0
	m.configScrollRow = 0
	m.configDirty = false
	m.configSavedMsg = ""
	return m
}

func (m Model) configEditorHeight() int {
	h := m.height - 9
	if h < 6 {
		h = 6
	}
	return h
}

func (m Model) ensureConfigScroll() {
	visible := m.configEditorHeight()
	if m.configCursorRow < m.configScrollRow {
		m.configScrollRow = m.configCursorRow
	}
	if m.configCursorRow >= m.configScrollRow+visible {
		m.configScrollRow = m.configCursorRow - visible + 1
	}
}

func (m Model) handleConfigKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	switch key {
	case "ctrl+s":
		return m, m.saveConfigEditor()
	case "R":
		return m.loadConfigEditor(), nil
	case "up":
		if m.configCursorRow > 0 {
			m.configCursorRow--
			m.configCursorCol = components.ClampCursorCol(m.configLines, m.configCursorRow, m.configCursorCol)
			m.ensureConfigScroll()
		}
		return m, nil
	case "down":
		if m.configCursorRow < len(m.configLines)-1 {
			m.configCursorRow++
			m.configCursorCol = components.ClampCursorCol(m.configLines, m.configCursorRow, m.configCursorCol)
			m.ensureConfigScroll()
		}
		return m, nil
	case "left":
		if m.configCursorCol > 0 {
			m.configCursorCol--
		} else if m.configCursorRow > 0 {
			m.configCursorRow--
			m.configCursorCol = utf8.RuneCountInString(m.configLines[m.configCursorRow])
			m.ensureConfigScroll()
		}
		return m, nil
	case "right":
		lineLen := utf8.RuneCountInString(m.configLines[m.configCursorRow])
		if m.configCursorCol < lineLen {
			m.configCursorCol++
		} else if m.configCursorRow < len(m.configLines)-1 {
			m.configCursorRow++
			m.configCursorCol = 0
			m.ensureConfigScroll()
		}
		return m, nil
	case "home":
		m.configCursorCol = 0
		return m, nil
	case "end":
		m.configCursorCol = utf8.RuneCountInString(m.configLines[m.configCursorRow])
		return m, nil
	case "pgup":
		step := m.configEditorHeight()
		if m.configCursorRow > step {
			m.configCursorRow -= step
		} else {
			m.configCursorRow = 0
		}
		m.ensureConfigScroll()
		return m, nil
	case "pgdown":
		step := m.configEditorHeight()
		if m.configCursorRow+step < len(m.configLines) {
			m.configCursorRow += step
		} else {
			m.configCursorRow = len(m.configLines) - 1
		}
		m.ensureConfigScroll()
		return m, nil
	case "backspace":
		m = m.configBackspace()
		return m, nil
	case "delete":
		m = m.configDelete()
		return m, nil
	case "enter":
		m = m.configInsertNewline()
		return m, nil
	case "tab":
		m = m.configInsertRune(' ')
		m = m.configInsertRune(' ')
		return m, nil
	}

	if msg.Text != "" {
		for _, r := range msg.Text {
			if r == '\n' || r == '\r' {
				continue
			}
			m = m.configInsertRune(r)
		}
		return m, nil
	}
	return m, nil
}

func (m Model) configInsertRune(r rune) Model {
	m.configDirty = true
	m.configSavedMsg = ""
	line := m.configLines[m.configCursorRow]
	runes := []rune(line)
	col := m.configCursorCol
	if col > len(runes) {
		col = len(runes)
	}
	runes = append(runes[:col], append([]rune{r}, runes[col:]...)...)
	m.configLines[m.configCursorRow] = string(runes)
	m.configCursorCol++
	return m
}

func (m Model) configBackspace() Model {
	m.configDirty = true
	m.configSavedMsg = ""
	if m.configCursorCol > 0 {
		line := m.configLines[m.configCursorRow]
		runes := []rune(line)
		col := m.configCursorCol - 1
		runes = append(runes[:col], runes[col+1:]...)
		m.configLines[m.configCursorRow] = string(runes)
		m.configCursorCol = col
		return m
	}
	if m.configCursorRow == 0 {
		return m
	}
	prev := m.configLines[m.configCursorRow-1]
	cur := m.configLines[m.configCursorRow]
	m.configLines[m.configCursorRow-1] = prev + cur
	m.configLines = append(m.configLines[:m.configCursorRow], m.configLines[m.configCursorRow+1:]...)
	m.configCursorRow--
	m.configCursorCol = utf8.RuneCountInString(prev)
	m.ensureConfigScroll()
	return m
}

func (m Model) configDelete() Model {
	m.configDirty = true
	m.configSavedMsg = ""
	line := m.configLines[m.configCursorRow]
	runes := []rune(line)
	col := m.configCursorCol
	if col < len(runes) {
		runes = append(runes[:col], runes[col+1:]...)
		m.configLines[m.configCursorRow] = string(runes)
		return m
	}
	if m.configCursorRow >= len(m.configLines)-1 {
		return m
	}
	next := m.configLines[m.configCursorRow+1]
	m.configLines[m.configCursorRow] = line + next
	m.configLines = append(m.configLines[:m.configCursorRow+1], m.configLines[m.configCursorRow+2:]...)
	m.ensureConfigScroll()
	return m
}

func (m Model) configInsertNewline() Model {
	m.configDirty = true
	m.configSavedMsg = ""
	line := m.configLines[m.configCursorRow]
	runes := []rune(line)
	col := m.configCursorCol
	before := string(runes[:col])
	after := string(runes[col:])
	m.configLines[m.configCursorRow] = before
	m.configLines = append(
		append(m.configLines[:m.configCursorRow+1], after),
		m.configLines[m.configCursorRow+1:]...,
	)
	m.configCursorRow++
	m.configCursorCol = 0
	m.ensureConfigScroll()
	return m
}

func (m Model) saveConfigEditor() tea.Cmd {
	path := m.deps.ConfigPath
	content := components.LinesToText(m.configLines)
	return func() tea.Msg {
		cfg, err := config.SaveRaw(path, content)
		if err != nil {
			return configEditorSaveMsg{err: err}
		}
		return configEditorSaveMsg{cfg: cfg}
	}
}

type configEditorSaveMsg struct {
	cfg config.Config
	err error
}

func (m Model) handleConfigEditorSave(msg configEditorSaveMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.configErr = msg.err.Error()
		return m, nil
	}
	m.configErr = ""
	m.configDirty = false
	m.configSavedMsg = "saved — settings applied"
	m.deps.Config = msg.cfg
	m.autoRecord = msg.cfg.AutoRecord
	m.audioMonitorWarn = m.deps.Audio.MonitorWarning(msg.cfg.Audio.SystemMonitor)
	return m, resolveDeviceLabelsCmd(m)
}

func (m Model) isTabSwitchKey(key string) bool {
	switch key {
	case "1", "2", "3", "4", "c", "d":
		return true
	default:
		return false
	}
}