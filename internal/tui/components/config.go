package components

import (
	"fmt"
	"strings"

	"anoted/internal/audio"
	"charm.land/lipgloss/v2"
)

const (
	configSidebarMinWidth  = 14
	configSidebarMaxWidth  = 20
	configSidebarLayoutMin = 72
)

var (
	configSidebarActiveStyle = lipgloss.NewStyle().
					Border(lipgloss.RoundedBorder()).
					BorderForeground(lipgloss.Color("63")).
					Bold(true).
					Foreground(lipgloss.Color("229")).
					Padding(0, 1)
	configSidebarInactiveStyle = lipgloss.NewStyle().
					Border(lipgloss.RoundedBorder()).
					BorderForeground(lipgloss.Color("238")).
					Foreground(lipgloss.Color("244")).
					Padding(0, 1)
)

// ConfigFieldRow is one row in the config menu.
type ConfigFieldRow struct {
	Label     string
	Value     string
	Selected  bool
	Kind      string // bool, enum, text, int, list, readonly
	ListItems []ConfigListItem
}

// ConfigListItem is one item in an expandable list field.
type ConfigListItem struct {
	Text     string
	Selected bool
}

// ConfigSectionPanel is one configuration section with its fields.
type ConfigSectionPanel struct {
	Label   string
	Focused bool
	Fields  []ConfigFieldRow
}

// ConfigMenuView renders the interactive Config tab.
type ConfigMenuView struct {
	Path             string
	Sections         []ConfigSectionPanel
	ModalOpen        bool
	ModalTitle       string
	ModalOptions     []string
	ModalCursor      int
	DevicePickerOpen bool
	DeviceTitle      string
	DeviceSection    AudioSection
	DeviceCatalog    audio.Catalog
	DeviceCursor     int
	DeviceLoading    bool
	DeviceErr        string
	SystemMonitor    string
	Microphone       string
	Editing          bool
	InputValue       string
	Width            int
	Height           int
}

func (v ConfigMenuView) View() string {
	base := v.renderBase()
	if v.DevicePickerOpen {
		h := v.overlayHeight()
		return FloatCenter(base, v.renderDeviceModal(), v.Width, h)
	}
	if !v.ModalOpen {
		return base
	}
	h := v.overlayHeight()
	return FloatCenter(base, v.renderModal(), v.Width, h)
}

func (v ConfigMenuView) renderBase() string {
	var b strings.Builder
	b.WriteString(subtleStyle.Render("File: " + v.Path))
	b.WriteString("\n\n")
	b.WriteString(v.renderAllSections())
	return b.String()
}

func (v ConfigMenuView) overlayHeight() int {
	h := v.Height - 8
	if h < 12 {
		h = 12
	}
	baseH := lipgloss.Height(v.renderBase())
	if baseH > h {
		h = baseH
	}
	return h
}

func (v ConfigMenuView) renderAllSections() string {
	if len(v.Sections) == 0 {
		return subtleStyle.Render("(no sections)")
	}
	if v.Width < configSidebarLayoutMin {
		return v.renderStackedSectionTabs()
	}
	sidebarW := configSidebarWidth(v.Width)
	contentW := v.Width - sidebarW - panelColumnGap
	if contentW < MinPanelWidth {
		return v.renderStackedSectionTabs()
	}

	sidebar := v.renderSidebar(sidebarW)
	content := v.renderContentPanel(v.activeSection(), contentW)
	left, right := EqualizeBoxHeights(sidebar, content)
	gap := strings.Repeat(" ", panelColumnGap)
	joined := lipgloss.JoinHorizontal(lipgloss.Top, left, gap, right)
	return PadLineBlock(joined, v.Width)
}

func configSidebarWidth(total int) int {
	switch {
	case total >= 100:
		return configSidebarMaxWidth
	case total >= 86:
		return 18
	default:
		return configSidebarMinWidth
	}
}

func (v ConfigMenuView) activeSection() ConfigSectionPanel {
	for _, sec := range v.Sections {
		if sec.Focused {
			return sec
		}
	}
	return v.Sections[0]
}

func (v ConfigMenuView) renderStackedSectionTabs() string {
	layout := NewPanelLayout(v.Width)
	labels := make([]string, len(v.Sections))
	active := 0
	for i, sec := range v.Sections {
		labels[i] = sec.Label
		if sec.Focused {
			active = i
		}
	}
	var b strings.Builder
	b.WriteString(SubTabBar(labels, active))
	b.WriteString("\n\n")
	b.WriteString(v.renderContentPanel(v.activeSection(), layout.FullWidth()))
	return b.String()
}

func (v ConfigMenuView) renderSidebar(width int) string {
	items := make([]string, 0, len(v.Sections))
	for _, sec := range v.Sections {
		items = append(items, v.renderSidebarItem(sec, width))
	}
	return strings.Join(items, "\n")
}

func (v ConfigMenuView) renderSidebarItem(sec ConfigSectionPanel, width int) string {
	label := strings.ToUpper(sec.Label)
	style := configSidebarInactiveStyle
	if sec.Focused {
		style = configSidebarActiveStyle
	}
	return style.Width(width).Align(lipgloss.Center).Render(label)
}

func (v ConfigMenuView) renderContentPanel(sec ConfigSectionPanel, width int) string {
	body := v.renderSectionFields(true, sec)
	return Box(strings.ToUpper(sec.Label), body, width)
}

func (v ConfigMenuView) renderSectionFields(focused bool, sec ConfigSectionPanel) string {
	if len(sec.Fields) == 0 {
		return subtleStyle.Render("(no fields)")
	}
	var lines []string
	for _, f := range sec.Fields {
		lines = append(lines, v.renderFieldLine(focused, f))
		if f.Kind == "list" {
			lines = append(lines, v.renderListItems(focused, f)...)
		}
	}
	return strings.Join(lines, "\n")
}

func (v ConfigMenuView) renderFieldLine(focused bool, f ConfigFieldRow) string {
	marker := "  "
	if focused && f.Selected {
		marker = "> "
	}
	label := labelStyle.Render(f.Label + ":")
	val := valueStyle.Render(f.Value)
	if f.Kind == "readonly" || f.Kind == "device" || f.Kind == "path" {
		val = subtleStyle.Render(f.Value)
	}
	if focused && f.Selected && v.Editing && f.Kind != "list" {
		if v.InputValue != "" {
			val = valueStyle.Render(v.InputValue + "_")
		} else {
			val = valueStyle.Render(f.Value + "_")
		}
	}
	return marker + label + " " + val
}

func (v ConfigMenuView) renderListItems(focused bool, f ConfigFieldRow) []string {
	var lines []string
	if len(f.ListItems) == 0 {
		hint := "      (empty)"
		if focused && f.Selected {
			hint += " — press a to add"
		}
		lines = append(lines, subtleStyle.Render(hint))
		return lines
	}
	for _, item := range f.ListItems {
		im := "      "
		if focused && item.Selected {
			im = "    > "
		}
		lines = append(lines, im+valueStyle.Render(item.Text))
	}
	if focused && f.Selected && v.Editing {
		prompt := "      new: "
		if v.InputValue != "" {
			prompt += v.InputValue
		}
		lines = append(lines, valueStyle.Render(prompt+"_"))
	}
	return lines
}

func (v ConfigMenuView) renderModal() string {
	var lines []string
	for i, opt := range v.ModalOptions {
		marker := "  "
		if i == v.ModalCursor {
			marker = "> "
		}
		lines = append(lines, marker+valueStyle.Render(opt))
	}
	lines = append(lines, "")
	lines = append(lines, subtleStyle.Render("↑↓ choose · Enter apply · Esc cancel"))
	body := strings.Join(lines, "\n")
	maxW := v.Width - 10
	if maxW < 28 {
		maxW = 28
	}
	if maxW > 48 {
		maxW = 48
	}
	return PickerModal(v.ModalTitle, body, maxW)
}

func (v ConfigMenuView) renderDeviceModal() string {
	var lines []string
	lines = append(lines, row("Selection", v.deviceSelectionLabel()))
	lines = append(lines, "")

	if v.DeviceLoading {
		lines = append(lines, subtleStyle.Render("  Loading devices…"))
	} else if v.DeviceErr != "" {
		lines = append(lines, errStyle.Render("  ✗ "+v.DeviceErr))
	} else {
		panel := AudioPanel{
			Catalog:       v.DeviceCatalog,
			Section:       v.DeviceSection,
			Cursor:        v.DeviceCursor,
			SystemMonitor: v.SystemMonitor,
			Microphone:    v.Microphone,
		}
		lines = append(lines, panel.renderDeviceList())
	}

	lines = append(lines, "")
	lines = append(lines, subtleStyle.Render("↑↓ choose · Enter apply · Esc cancel"))
	body := strings.Join(lines, "\n")
	maxW := v.Width - 10
	if maxW < 36 {
		maxW = 36
	}
	if maxW > 72 {
		maxW = 72
	}
	return PickerModal(v.DeviceTitle, body, maxW)
}

func (v ConfigMenuView) deviceSelectionLabel() string {
	if v.DeviceSection == AudioSectionOutput {
		if v.SystemMonitor == "" {
			return "(auto)"
		}
		return truncate(v.SystemMonitor, v.Width-14)
	}
	if v.Microphone == "" {
		return "(auto)"
	}
	return truncate(v.Microphone, v.Width-14)
}

// ConfigFooterMode selects footer shortcuts on the Config screen.
type ConfigFooterMode int

const (
	ConfigFooterNormal ConfigFooterMode = iota
	ConfigFooterModal
	ConfigFooterDevicePicker
	ConfigFooterEditing
	ConfigFooterPathField
)

// FooterForConfig returns context-sensitive footer for the Config tab.
func FooterForConfig(mode ConfigFooterMode, savedMsg, errMsg string, width int) string {
	var hints string
	switch mode {
	case ConfigFooterModal, ConfigFooterDevicePicker:
		hints = JoinFooter(
			FooterHint("↑↓", "choose"),
			FooterHint("Enter", "apply"),
			FooterHint("Esc", "cancel"),
			FooterHint("q", "quit"),
		)
	case ConfigFooterPathField:
		hints = JoinFooter(
			FooterHint("Enter", "browse"),
			FooterHint("e", "edit"),
			FooterHint("c", "clear"),
			FooterHint("q", "quit"),
		)
	case ConfigFooterEditing:
		hints = JoinFooter(
			FooterHint("Enter", "confirm"),
			FooterHint("Esc", "cancel"),
			FooterHint("q", "quit"),
		)
	default:
		hints = JoinFooter(
			FooterHint("←→", "section"),
			FooterHint("↑↓", "navigate"),
			FooterHint("Enter", "edit"),
			FooterHint("a/d", "patterns"),
			FooterHint("R", "reload"),
			FooterHint("q", "quit"),
		)
	}

	if errMsg != "" {
		return FooterWithTrailingStatus(hints, errStyle.Render("✗ "+errMsg), width)
	}
	if savedMsg != "" {
		return FooterWithTrailingStatus(hints, okStyle.Render("✓ "+savedMsg), width)
	}
	return hints
}

// FormatConfigBool displays a bool for the menu.
func FormatConfigBool(b bool) string {
	return fmt.Sprintf("%v", b)
}
