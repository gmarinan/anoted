package tui

import (
	"fmt"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
	"meetctl/internal/config"
)

const configSectionCount = 6

var configSectionLabels = []string{
	"General", "Audio", "Detection", "Transcription", "Desktop", "Privacy",
}

type cfgFieldKind int

const (
	fieldBool cfgFieldKind = iota
	fieldEnum
	fieldText
	fieldInt
	fieldList
	fieldReadonly
)

type cfgField struct {
	label   string
	kind    cfgFieldKind
	options []string
	get     func(c config.Config) string
	set     func(c *config.Config, v string) error
	list    func(c config.Config) []string
	addItem func(c *config.Config, v string)
	delItem func(c *config.Config, i int)
}

func cfgFields(section int) []cfgField {
	switch section {
	case 0:
		return generalCfgFields()
	case 1:
		return audioCfgFields()
	case 2:
		return detectionCfgFields()
	case 3:
		return transcriptionCfgFields()
	case 4:
		return desktopCfgFields()
	case 5:
		return privacyCfgFields()
	default:
		return nil
	}
}

func generalCfgFields() []cfgField {
	return []cfgField{
		{
			label: "output_dir",
			kind:  fieldText,
			get:   func(c config.Config) string { return c.OutputDir },
			set:   func(c *config.Config, v string) error { c.OutputDir = v; return nil },
		},
		{
			label: "auto_record",
			kind:  fieldBool,
			get: func(c config.Config) string {
				return fmt.Sprintf("%v", c.AutoRecord)
			},
			set: func(c *config.Config, v string) error {
				b, err := strconv.ParseBool(v)
				if err != nil {
					return err
				}
				c.AutoRecord = b
				return nil
			},
		},
		{
			label: "auto_record_requires_confirmation",
			kind:  fieldBool,
			get: func(c config.Config) string {
				return fmt.Sprintf("%v", c.AutoRecordRequiresConfirmation)
			},
			set: func(c *config.Config, v string) error {
				b, err := strconv.ParseBool(v)
				if err != nil {
					return err
				}
				c.AutoRecordRequiresConfirmation = b
				return nil
			},
		},
	}
}

func audioCfgFields() []cfgField {
	return []cfgField{
		{
			label: "sample_rate",
			kind:  fieldInt,
			get: func(c config.Config) string {
				return strconv.Itoa(c.Audio.SampleRate)
			},
			set: func(c *config.Config, v string) error {
				n, err := strconv.Atoi(v)
				if err != nil {
					return fmt.Errorf("invalid sample_rate: %w", err)
				}
				c.Audio.SampleRate = n
				return nil
			},
		},
		{
			label:   "channels",
			kind:    fieldEnum,
			options: []string{"1", "2"},
			get: func(c config.Config) string {
				return strconv.Itoa(c.Audio.Channels)
			},
			set: func(c *config.Config, v string) error {
				n, err := strconv.Atoi(v)
				if err != nil {
					return err
				}
				c.Audio.Channels = n
				return nil
			},
		},
		{
			label: "system_monitor",
			kind:  fieldReadonly,
			get: func(c config.Config) string {
				if c.Audio.SystemMonitor == "" {
					return "(auto — choose on Home tab)"
				}
				return c.Audio.SystemMonitor
			},
		},
		{
			label: "microphone",
			kind:  fieldReadonly,
			get: func(c config.Config) string {
				if c.Audio.Microphone == "" {
					return "(auto — choose on Home tab)"
				}
				return c.Audio.Microphone
			},
		},
		{
			label: "linux_backend_priority",
			kind:  fieldReadonly,
			get: func(c config.Config) string {
				return strings.Join(c.Audio.LinuxBackendPriority, ", ")
			},
		},
		{
			label: "windows_backend_priority",
			kind:  fieldReadonly,
			get: func(c config.Config) string {
				return strings.Join(c.Audio.WindowsBackendPriority, ", ")
			},
		},
	}
}

func detectionCfgFields() []cfgField {
	fields := []cfgField{
		{
			label: "poll_interval_ms",
			kind:  fieldInt,
			get: func(c config.Config) string {
				return strconv.Itoa(c.Detection.PollIntervalMS)
			},
			set: func(c *config.Config, v string) error {
				n, err := strconv.Atoi(v)
				if err != nil {
					return fmt.Errorf("invalid poll_interval_ms: %w", err)
				}
				c.Detection.PollIntervalMS = n
				return nil
			},
		},
		{
			label:   "mode",
			kind:    fieldEnum,
			options: []string{"mic", "window", "both", "none"},
			get:     func(c config.Config) string { return c.Detection.Mode },
			set:     func(c *config.Config, v string) error { c.Detection.Mode = v; return nil },
		},
		{
			label:   "window_tool",
			kind:    fieldEnum,
			options: []string{"auto", "xdotool", "wmctrl", "none"},
			get:     func(c config.Config) string { return c.Detection.WindowTool },
			set:     func(c *config.Config, v string) error { c.Detection.WindowTool = v; return nil },
		},
	}
	fields = append(fields, providerListField("google_meet", "Google Meet patterns"))
	fields = append(fields, providerListField("teams", "Teams patterns"))
	return fields
}

func providerListField(id, label string) cfgField {
	return cfgField{
		label: label,
		kind:  fieldList,
		list: func(c config.Config) []string {
			if c.Detection.Providers == nil {
				return nil
			}
			p, ok := c.Detection.Providers[id]
			if !ok {
				return nil
			}
			return append([]string(nil), p.Patterns...)
		},
		addItem: func(c *config.Config, v string) {
			if c.Detection.Providers == nil {
				c.Detection.Providers = make(map[string]config.ProviderConfig)
			}
			p := c.Detection.Providers[id]
			p.Patterns = append(p.Patterns, v)
			c.Detection.Providers[id] = p
		},
		delItem: func(c *config.Config, i int) {
			if c.Detection.Providers == nil {
				return
			}
			p, ok := c.Detection.Providers[id]
			if !ok || i < 0 || i >= len(p.Patterns) {
				return
			}
			p.Patterns = append(p.Patterns[:i], p.Patterns[i+1:]...)
			c.Detection.Providers[id] = p
		},
	}
}

func transcriptionCfgFields() []cfgField {
	return []cfgField{
		{
			label: "auto_after_recording",
			kind:  fieldBool,
			get: func(c config.Config) string {
				return fmt.Sprintf("%v", c.Transcription.AutoAfterRecording)
			},
			set: func(c *config.Config, v string) error {
				b, err := strconv.ParseBool(v)
				if err != nil {
					return err
				}
				c.Transcription.AutoAfterRecording = b
				return nil
			},
		},
		{
			label: "binary",
			kind:  fieldText,
			get: func(c config.Config) string {
				if c.Transcription.Binary == "" {
					return "(auto-detect)"
				}
				return c.Transcription.Binary
			},
			set: func(c *config.Config, v string) error {
				if v == "(auto-detect)" {
					v = ""
				}
				c.Transcription.Binary = v
				return nil
			},
		},
		{
			label:   "backend",
			kind:    fieldEnum,
			options: []string{"auto", "openai-whisper", "whisper-cpp"},
			get:     func(c config.Config) string { return c.Transcription.Backend },
			set:     func(c *config.Config, v string) error { c.Transcription.Backend = v; return nil },
		},
		{
			label:   "model",
			kind:    fieldEnum,
			options: []string{"tiny", "base", "small", "medium", "large", "turbo"},
			get:     func(c config.Config) string { return c.Transcription.Model },
			set:     func(c *config.Config, v string) error { c.Transcription.Model = v; return nil },
		},
		{
			label: "language",
			kind:  fieldText,
			get: func(c config.Config) string {
				if c.Transcription.Language == "" {
					return "(auto-detect)"
				}
				return c.Transcription.Language
			},
			set: func(c *config.Config, v string) error {
				if v == "(auto-detect)" {
					v = ""
				}
				c.Transcription.Language = v
				return nil
			},
		},
		{
			label:   "device",
			kind:    fieldEnum,
			options: []string{"auto", "cpu", "cuda"},
			get:     func(c config.Config) string { return c.Transcription.Device },
			set:     func(c *config.Config, v string) error { c.Transcription.Device = v; return nil },
		},
		{
			label: "gpu_layers",
			kind:  fieldInt,
			get: func(c config.Config) string {
				return strconv.Itoa(c.Transcription.GPULayers)
			},
			set: func(c *config.Config, v string) error {
				n, err := strconv.Atoi(v)
				if err != nil {
					return fmt.Errorf("invalid gpu_layers: %w", err)
				}
				c.Transcription.GPULayers = n
				return nil
			},
		},
		{
			label: "model_path",
			kind:  fieldText,
			get: func(c config.Config) string {
				if c.Transcription.ModelPath == "" {
					return "(default)"
				}
				return c.Transcription.ModelPath
			},
			set: func(c *config.Config, v string) error {
				if v == "(default)" {
					v = ""
				}
				c.Transcription.ModelPath = v
				return nil
			},
		},
	}
}

func desktopCfgFields() []cfgField {
	return []cfgField{
		{
			label:   "opener",
			kind:    fieldEnum,
			options: []string{"auto", "xdg-open", "dolphin", "nautilus", "thunar", "pcmanfm", "nemo", "caja", "custom"},
			get:     func(c config.Config) string { return c.Desktop.Opener },
			set:     func(c *config.Config, v string) error { c.Desktop.Opener = v; return nil },
		},
		{
			label: "open_command",
			kind:  fieldText,
			get: func(c config.Config) string {
				if len(c.Desktop.OpenCommand) == 0 {
					return "(empty)"
				}
				return strings.Join(c.Desktop.OpenCommand, " ")
			},
			set: func(c *config.Config, v string) error {
				if v == "(empty)" || strings.TrimSpace(v) == "" {
					c.Desktop.OpenCommand = nil
					return nil
				}
				c.Desktop.OpenCommand = strings.Fields(v)
				return nil
			},
		},
		{
			label: "file_opener",
			kind:  fieldText,
			get: func(c config.Config) string {
				if c.Desktop.FileOpener == "" {
					return "(default)"
				}
				return c.Desktop.FileOpener
			},
			set: func(c *config.Config, v string) error {
				if v == "(default)" {
					v = ""
				}
				c.Desktop.FileOpener = v
				return nil
			},
		},
	}
}

func privacyCfgFields() []cfgField {
	return []cfgField{
		{
			label: "show_recording_indicator",
			kind:  fieldBool,
			get: func(c config.Config) string {
				return fmt.Sprintf("%v", c.Privacy.ShowRecordingIndicator)
			},
			set: func(c *config.Config, v string) error {
				b, err := strconv.ParseBool(v)
				if err != nil {
					return err
				}
				c.Privacy.ShowRecordingIndicator = b
				return nil
			},
		},
		{
			label: "require_manual_consent_first_run",
			kind:  fieldBool,
			get: func(c config.Config) string {
				return fmt.Sprintf("%v", c.Privacy.RequireManualConsentFirstRun)
			},
			set: func(c *config.Config, v string) error {
				b, err := strconv.ParseBool(v)
				if err != nil {
					return err
				}
				c.Privacy.RequireManualConsentFirstRun = b
				return nil
			},
		},
	}
}

func (m Model) currentCfgFields() []cfgField {
	return cfgFields(m.configSection)
}

func (m Model) currentCfgField() (cfgField, bool) {
	fields := m.currentCfgFields()
	if m.configCursor < 0 || m.configCursor >= len(fields) {
		return cfgField{}, false
	}
	return fields[m.configCursor], true
}

func cfgFieldValue(f cfgField, cfg config.Config) string {
	if f.get != nil {
		return f.get(cfg)
	}
	if f.kind == fieldList && f.list != nil {
		n := len(f.list(cfg))
		if n == 1 {
			return "1 pattern"
		}
		return fmt.Sprintf("%d patterns", n)
	}
	return ""
}

func (m Model) initConfigMenu() Model {
	m.configSection = 0
	m.configCursor = 0
	m.configListCursor = 0
	m.configEditing = false
	m.configInput = ""
	m.configModalOpen = false
	m.configModalCursor = 0
	m.configModalOptions = nil
	m.configErr = ""
	m.configSavedMsg = ""
	return m
}

func (m Model) reloadConfigFromDisk() Model {
	cfg, err := config.Load(m.deps.ConfigPath)
	if err != nil {
		m.configErr = err.Error()
		return m
	}
	m.deps.Config = cfg
	m.autoRecord = cfg.AutoRecord
	m.audioMonitorWarn = m.deps.Audio.MonitorWarning(cfg.Audio.SystemMonitor)
	m.configErr = ""
	m.configSavedMsg = "reloaded from disk"
	return m
}

func (m Model) clampConfigCursor() Model {
	fields := m.currentCfgFields()
	if len(fields) == 0 {
		m.configCursor = 0
		return m
	}
	if m.configCursor >= len(fields) {
		m.configCursor = len(fields) - 1
	}
	if m.configCursor < 0 {
		m.configCursor = 0
	}
	f := fields[m.configCursor]
	if f.kind == fieldList {
		items := f.list(m.deps.Config)
		if len(items) == 0 {
			m.configListCursor = 0
		} else if m.configListCursor >= len(items) {
			m.configListCursor = len(items) - 1
		}
		if m.configListCursor < 0 {
			m.configListCursor = 0
		}
	}
	return m
}

func (m Model) handleConfigKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	if m.configModalOpen {
		return m.handleConfigModalKey(key)
	}
	if m.configEditing {
		return m.handleConfigInputKey(msg)
	}

	switch key {
	case "tab":
		m.configSection = (m.configSection + 1) % configSectionCount
		m.configCursor = 0
		m.configListCursor = 0
		return m, nil
	case "shift+tab":
		m.configSection--
		if m.configSection < 0 {
			m.configSection = configSectionCount - 1
		}
		m.configCursor = 0
		m.configListCursor = 0
		return m, nil
	case "R":
		return m.reloadConfigFromDisk(), resolveDeviceLabelsCmd(m)
	case "up", "k":
		return m.configNavUp(), nil
	case "down", "j":
		return m.configNavDown(), nil
	case "enter", " ":
		return m.activateConfigField()
	case "a":
		return m.startConfigListAdd()
	case "d", "x":
		return m.deleteConfigListItem()
	}
	return m, nil
}

func (m Model) handleConfigModalKey(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "esc":
		m.configModalOpen = false
		m.configModalOptions = nil
		return m, nil
	case "up", "k":
		if m.configModalCursor > 0 {
			m.configModalCursor--
		}
		return m, nil
	case "down", "j":
		if m.configModalCursor < len(m.configModalOptions)-1 {
			m.configModalCursor++
		}
		return m, nil
	case "enter", " ":
		if m.configModalCursor < 0 || m.configModalCursor >= len(m.configModalOptions) {
			return m, nil
		}
		val := m.configModalOptions[m.configModalCursor]
		m.configModalOpen = false
		m.configModalOptions = nil
		return m, m.applyConfigValue(val)
	}
	return m, nil
}

func (m Model) handleConfigInputKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	switch key {
	case "esc":
		m.configEditing = false
		m.configInput = ""
		return m, nil
	case "enter":
		val := strings.TrimSpace(m.configInput)
		m.configEditing = false
		m.configInput = ""
		if val == "" {
			return m, nil
		}
		f, ok := m.currentCfgField()
		if ok && f.kind == fieldList {
			return m, m.applyConfigListAdd(val)
		}
		return m, m.applyConfigValue(val)
	case "backspace":
		if len(m.configInput) > 0 {
			m.configInput = m.configInput[:len(m.configInput)-1]
		}
		return m, nil
	}
	if msg.Text != "" {
		for _, r := range msg.Text {
			if r == '\n' || r == '\r' {
				continue
			}
			f, ok := m.currentCfgField()
			if ok && f.kind == fieldInt {
				if r < '0' || r > '9' {
					continue
				}
			}
			m.configInput += string(r)
		}
	}
	return m, nil
}

func (m Model) activateConfigField() (tea.Model, tea.Cmd) {
	f, ok := m.currentCfgField()
	if !ok || f.kind == fieldReadonly {
		return m, nil
	}

	switch f.kind {
	case fieldBool:
		cur := f.get(m.deps.Config)
		next := "true"
		if cur == "true" {
			next = "false"
		}
		return m, m.applyConfigValue(next)
	case fieldEnum:
		m.configModalOpen = true
		m.configModalOptions = append([]string(nil), f.options...)
		m.configModalCursor = 0
		cur := f.get(m.deps.Config)
		for i, opt := range f.options {
			if opt == cur {
				m.configModalCursor = i
				break
			}
		}
		return m, nil
	case fieldText, fieldInt:
		m.configEditing = true
		val := f.get(m.deps.Config)
		if val == "(auto-detect)" || val == "(default)" || val == "(empty)" {
			m.configInput = ""
		} else {
			m.configInput = val
		}
		return m, nil
	case fieldList:
		items := f.list(m.deps.Config)
		if len(items) == 0 {
			return m.startConfigListAdd()
		}
		return m, nil
	}
	return m, nil
}

func (m Model) startConfigListAdd() (tea.Model, tea.Cmd) {
	f, ok := m.currentCfgField()
	if !ok || f.kind != fieldList {
		return m, nil
	}
	m.configEditing = true
	m.configInput = ""
	return m, nil
}

func (m Model) deleteConfigListItem() (tea.Model, tea.Cmd) {
	f, ok := m.currentCfgField()
	if !ok || f.kind != fieldList || f.delItem == nil {
		return m, nil
	}
	items := f.list(m.deps.Config)
	if len(items) == 0 || m.configListCursor < 0 || m.configListCursor >= len(items) {
		return m, nil
	}
	idx := m.configListCursor
	cfg := m.deps.Config
	f.delItem(&cfg, idx)
	return m, m.saveConfigCmd(cfg)
}

func (m Model) applyConfigListAdd(val string) tea.Cmd {
	f, ok := m.currentCfgField()
	if !ok || f.kind != fieldList || f.addItem == nil {
		return nil
	}
	cfg := m.deps.Config
	f.addItem(&cfg, val)
	return m.saveConfigCmd(cfg)
}

func (m Model) applyConfigValue(val string) tea.Cmd {
	f, ok := m.currentCfgField()
	if !ok || f.set == nil {
		return nil
	}
	cfg := m.deps.Config
	if err := f.set(&cfg, val); err != nil {
		return func() tea.Msg {
			return configMenuSaveMsg{err: err}
		}
	}
	return m.saveConfigCmd(cfg)
}

func (m Model) saveConfigCmd(cfg config.Config) tea.Cmd {
	path := m.deps.ConfigPath
	return func() tea.Msg {
		if err := config.Save(path, cfg); err != nil {
			return configMenuSaveMsg{err: err}
		}
		return configMenuSaveMsg{cfg: cfg}
	}
}

type configMenuSaveMsg struct {
	cfg config.Config
	err error
}

func (m Model) handleConfigMenuSave(msg configMenuSaveMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.configErr = msg.err.Error()
		m.configSavedMsg = ""
		return m, nil
	}
	m.configErr = ""
	m.configSavedMsg = "saved"
	m.deps.Config = msg.cfg
	m.autoRecord = msg.cfg.AutoRecord
	m.audioMonitorWarn = m.deps.Audio.MonitorWarning(msg.cfg.Audio.SystemMonitor)
	m = m.clampConfigCursor()
	return m, resolveDeviceLabelsCmd(m)
}

func (m Model) configNavUp() Model {
	f, ok := m.currentCfgField()
	if ok && f.kind == fieldList {
		items := f.list(m.deps.Config)
		if len(items) > 0 && m.configListCursor > 0 {
			m.configListCursor--
			return m
		}
	}
	if m.configCursor > 0 {
		m.configCursor--
		m.configListCursor = 0
	}
	return m
}

func (m Model) configNavDown() Model {
	f, ok := m.currentCfgField()
	if ok && f.kind == fieldList {
		items := f.list(m.deps.Config)
		if len(items) > 0 && m.configListCursor < len(items)-1 {
			m.configListCursor++
			return m
		}
	}
	fields := m.currentCfgFields()
	if m.configCursor < len(fields)-1 {
		m.configCursor++
		m.configListCursor = 0
	}
	return m
}

func (m Model) isTabSwitchKey(key string) bool {
	switch key {
	case "1", "2", "3", "4":
		return true
	default:
		return false
	}
}
