package tui

import (
	"fmt"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
	"anoted/internal/config"
	"anoted/internal/tui/components"
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
	fieldDevice
)

type cfgField struct {
	label         string
	kind          cfgFieldKind
	options       []string
	deviceSection components.AudioSection
	editable      func(c config.Config) bool
	get           func(c config.Config) string
	set           func(c *config.Config, v string) error
	list          func(c config.Config) []string
	addItem       func(c *config.Config, v string)
	delItem       func(c *config.Config, i int)
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
			label:   "level_meter",
			kind:    fieldEnum,
			options: config.LevelPresetOptions(),
			get: func(c config.Config) string {
				if c.Audio.LevelPreset == "" {
					return config.LevelPresetLabel(config.InferLevelPreset(
						c.Audio.LevelLatencyMsec, c.Audio.LevelProcessTimeMsec, c.Audio.LevelUITickMS,
					))
				}
				return config.LevelPresetLabel(c.Audio.LevelPreset)
			},
			set: func(c *config.Config, v string) error {
				return config.ApplyLevelPreset(c, config.LevelPresetID(v))
			},
		},
		{
			label: "level_latency_msec",
			kind:  fieldInt,
			editable: func(c config.Config) bool {
				return c.Audio.LevelPreset == config.LevelPresetCustom
			},
			get: func(c config.Config) string {
				return strconv.Itoa(c.Audio.LevelLatencyMsec)
			},
			set: func(c *config.Config, v string) error {
				n, err := strconv.Atoi(v)
				if err != nil {
					return fmt.Errorf("invalid level_latency_msec: %w", err)
				}
				if n < 10 {
					return fmt.Errorf("level_latency_msec must be >= 10")
				}
				c.Audio.LevelLatencyMsec = n
				if c.Audio.LevelProcessTimeMsec > n {
					c.Audio.LevelProcessTimeMsec = n
				}
				config.MarkLevelPresetCustom(c)
				return nil
			},
		},
		{
			label: "level_process_time_msec",
			kind:  fieldInt,
			editable: func(c config.Config) bool {
				return c.Audio.LevelPreset == config.LevelPresetCustom
			},
			get: func(c config.Config) string {
				return strconv.Itoa(c.Audio.LevelProcessTimeMsec)
			},
			set: func(c *config.Config, v string) error {
				n, err := strconv.Atoi(v)
				if err != nil {
					return fmt.Errorf("invalid level_process_time_msec: %w", err)
				}
				if n < 5 {
					return fmt.Errorf("level_process_time_msec must be >= 5")
				}
				if c.Audio.LevelLatencyMsec > 0 && n > c.Audio.LevelLatencyMsec {
					return fmt.Errorf("level_process_time_msec must be <= level_latency_msec")
				}
				c.Audio.LevelProcessTimeMsec = n
				config.MarkLevelPresetCustom(c)
				return nil
			},
		},
		{
			label: "level_ui_tick_ms",
			kind:  fieldInt,
			editable: func(c config.Config) bool {
				return c.Audio.LevelPreset == config.LevelPresetCustom
			},
			get: func(c config.Config) string {
				return strconv.Itoa(c.Audio.LevelUITickMS)
			},
			set: func(c *config.Config, v string) error {
				n, err := strconv.Atoi(v)
				if err != nil {
					return fmt.Errorf("invalid level_ui_tick_ms: %w", err)
				}
				if n < 16 {
					return fmt.Errorf("level_ui_tick_ms must be >= 16")
				}
				if n > 2000 {
					return fmt.Errorf("level_ui_tick_ms must be <= 2000")
				}
				c.Audio.LevelUITickMS = n
				config.MarkLevelPresetCustom(c)
				return nil
			},
		},
		{
			label:         "system_monitor",
			kind:          fieldDevice,
			deviceSection: components.AudioSectionOutput,
			get: func(c config.Config) string {
				if c.Audio.SystemMonitor == "" {
					return "(auto)"
				}
				return c.Audio.SystemMonitor
			},
		},
		{
			label:         "microphone",
			kind:          fieldDevice,
			deviceSection: components.AudioSectionMic,
			get: func(c config.Config) string {
				if c.Audio.Microphone == "" {
					return "(auto)"
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
	m.configDevicePickerOpen = false
	m.configDeviceLoading = false
	m.configDeviceErr = ""
	m.configDeviceCursor = 0
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

	if m.configDevicePickerOpen {
		return m.handleConfigDeviceModalKey(key)
	}
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
	k := msg.Key()
	if k.Text != "" {
		for _, r := range k.Text {
			m = m.appendConfigInputRune(r)
		}
		return m, nil
	}
	if len(key) == 1 {
		m = m.appendConfigInputRune(rune(key[0]))
	}
	return m, nil
}

func (m Model) appendConfigInputRune(r rune) Model {
	if r == '\n' || r == '\r' {
		return m
	}
	f, ok := m.currentCfgField()
	if ok && f.kind == fieldInt {
		if r < '0' || r > '9' {
			return m
		}
	}
	m.configInput += string(r)
	return m
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
		if f.editable != nil && !f.editable(m.deps.Config) {
			return m, nil
		}
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
	case fieldDevice:
		return m.openConfigDevicePicker(f.deviceSection)
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
	var cmds []tea.Cmd
	cmds = append(cmds, resolveDeviceLabelsCmd(m))
	if m.screen == ScreenMain {
		m.systemBands = nil
		m.micBands = nil
		m.levelGen++
		if config.LevelMeterEnabled(msg.cfg) {
			cmds = append(cmds, m.startSystemLevelCmd(), m.scheduleLevelTick(m.levelGen))
		} else {
			cmds = append(cmds, m.stopSystemLevelCmd(), m.stopMicLevelCmd())
		}
	}
	return m, tea.Batch(cmds...)
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

func (m Model) configAbsorbsKeys() bool {
	return m.configEditing || m.configModalOpen || m.configDevicePickerOpen
}

func (m Model) isTabSwitchKey(key string) bool {
	switch key {
	case "1", "2", "3", "4":
		return true
	default:
		return false
	}
}
