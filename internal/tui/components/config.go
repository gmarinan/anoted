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

// Sidebar styles live in theme.go with the rest of the palette.

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
		h := v.overlayHeight(base)
		return FloatCenter(base, v.renderDeviceModal(), v.Width, h)
	}
	if !v.ModalOpen {
		return base
	}
	h := v.overlayHeight(base)
	return FloatCenter(base, v.renderModal(), v.Width, h)
}

func (v ConfigMenuView) renderBase() string {
	var b strings.Builder
	b.WriteString(subtleStyle.Render("File: " + v.Path))
	b.WriteString("\n\n")
	b.WriteString(v.renderAllSections())
	return b.String()
}

// overlayHeight takes the already-rendered base so opening a modal does not
// render the whole screen twice (HomeView does the same).
func (v ConfigMenuView) overlayHeight(base string) int {
	h := v.Height - 8
	if h < 12 {
		h = 12
	}
	baseH := lipgloss.Height(base)
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
	// No right-padding here: PadView pads every line of the final frame once.
	return lipgloss.JoinHorizontal(lipgloss.Top, left, padSpaces(panelColumnGap), right)
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
	var b strings.Builder
	b.WriteString(v.stackedTabBar())
	b.WriteString("\n\n")
	b.WriteString(v.renderContentPanel(v.activeSection(), NewPanelLayout(v.Width).FullWidth()))
	return b.String()
}

// configSidebarChrome is the frame around the sidebar: header, tab bar, the
// File line and the footer.
const configSidebarChrome = 9

// stacked reports whether sections are tabs above the panel, not a sidebar.
func (v ConfigMenuView) stacked() bool {
	if v.Width < configSidebarLayoutMin {
		return true
	}
	return v.Width-configSidebarWidth(v.Width)-panelColumnGap < MinPanelWidth
}

func (v ConfigMenuView) stackedTabBar() string {
	labels := make([]string, len(v.Sections))
	active := 0
	for i, sec := range v.Sections {
		labels[i] = sec.Label
		if sec.Focused {
			active = i
		}
	}
	return SubTabBarWrapped(labels, active, NewPanelLayout(v.Width).FullWidth())
}

func (v ConfigMenuView) renderSidebar(width int) string {
	// Each boxed item takes three rows; with eight sections that no longer
	// fits a 30-row terminal, so fall back to one line per section.
	if v.Height > 0 && v.Height < 3*len(v.Sections)+configSidebarChrome {
		return v.renderCompactSidebar(width)
	}
	items := make([]string, 0, len(v.Sections))
	for _, sec := range v.Sections {
		items = append(items, v.renderSidebarItem(sec, width))
	}
	return strings.Join(items, "\n")
}

func (v ConfigMenuView) renderCompactSidebar(width int) string {
	inner := width - 4
	lines := make([]string, 0, len(v.Sections))
	for _, sec := range v.Sections {
		label := strings.ToUpper(sec.Label)
		if sec.Focused {
			lines = append(lines, selRowStyle.Render(padCell("▸ "+label, inner)))
			continue
		}
		lines = append(lines, subtleStyle.Render(padCell("  "+label, inner)))
	}
	// Not Box: it widens anything under MinPanelWidth, and the sidebar is narrower.
	return configSidebarInactiveStyle.Width(width).Render(strings.Join(lines, "\n"))
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
	body := v.renderSectionFields(true, sec, width-4)
	return Box(strings.ToUpper(sec.Label), body, width)
}

func (v ConfigMenuView) renderSectionFields(focused bool, sec ConfigSectionPanel, innerW int) string {
	if len(sec.Fields) == 0 {
		return subtleStyle.Render("(no fields)")
	}
	var lines []string
	cursorLine := 0
	for _, f := range sec.Fields {
		if f.Selected {
			cursorLine = len(lines)
		}
		lines = append(lines, v.renderFieldLine(focused, f, innerW))
		if f.Kind == "list" {
			lines = append(lines, v.renderListItems(focused, f)...)
		}
	}
	return strings.Join(v.windowFieldLines(lines, cursorLine), "\n")
}

// windowFieldLines keeps the selected field on screen.
//
// Transcription has fourteen fields and two of them expand into a line per list
// entry, so on a short terminal the cursor could move to a field that was not
// being drawn — the user pressed Enter on something they could not see.
func (v ConfigMenuView) windowFieldLines(lines []string, cursorLine int) []string {
	height := v.Height
	if v.stacked() && height > 0 {
		// The wrapped section tabs and the blank row under them sit above
		// the panel in this layout.
		height -= lipgloss.Height(v.stackedTabBar()) + 1
	}
	maxRows := configFieldRows(height)
	if maxRows <= 0 || len(lines) <= maxRows {
		return lines
	}
	start := cursorLine - maxRows/2
	if start < 0 {
		start = 0
	}
	if start+maxRows > len(lines) {
		start = len(lines) - maxRows
	}
	out := make([]string, 0, maxRows+1)
	out = append(out, lines[start:start+maxRows]...)
	out = append(out, subtleStyle.Render(
		fmt.Sprintf("  %d/%d  ↑↓ to scroll", cursorLine+1, len(lines))))
	return out
}

// configFieldRows budgets vertical space for the field list.
func configFieldRows(height int) int {
	const chrome = 10
	n := height - chrome
	switch {
	case height <= 0:
		return 0 // unknown size: draw everything, as before
	case n < 5:
		return 5
	case n > 30:
		return 30
	default:
		return n
	}
}

func (v ConfigMenuView) renderFieldLine(focused bool, f ConfigFieldRow, innerW int) string {
	if focused && f.Selected {
		// Full-width selection bar, matching the sessions table. Plain text
		// only: the bar's own colors carry the row, and any inner escapes
		// would cut the background short at their first reset.
		val := f.Value
		if v.Editing && f.Kind != "list" {
			if v.InputValue != "" {
				val = v.InputValue + "_"
			} else {
				val = f.Value + "_"
			}
		}
		return selRowStyle.Render(padCell("▸ "+f.Label+": "+val, innerW))
	}
	label := labelStyle.Render(f.Label + ":")
	val := valueStyle.Render(f.Value)
	if f.Kind == "readonly" || f.Kind == "device" || f.Kind == "path" {
		val = subtleStyle.Render(f.Value)
	}
	// Cut, do not wrap: a wrapped value spills onto a row of its own and
	// reads as the next field. The selected row above is cut the same way.
	return truncate("  "+label+" "+val, innerW)
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
			// Leave room for the modal chrome, the title and the hint line.
			MaxRows: deviceModalRows(v.Height),
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
	ConfigFooterModels
)

// FooterForConfig returns context-sensitive footer for the Config tab.
func FooterForConfig(mode ConfigFooterMode, savedMsg, errMsg string, width int) string {
	var hints []string
	switch mode {
	case ConfigFooterModal, ConfigFooterDevicePicker:
		hints = []string{
			FooterHint("↑↓", "choose"),
			FooterHint("Enter", "apply"),
			FooterHint("Esc", "cancel"),
			FooterHint("q", "quit"),
		}
	case ConfigFooterPathField:
		hints = []string{
			FooterHint("Enter", "browse"),
			FooterHint("e", "edit"),
			FooterHint("c", "clear"),
			FooterHint("q", "quit"),
		}
	case ConfigFooterModels:
		hints = []string{
			FooterHint("d", "delete"),
			FooterHint("←→", "section"),
			FooterHint("↑↓", "navigate"),
			FooterHint("R", "rescan"),
			FooterHint("q", "quit"),
		}
	case ConfigFooterEditing:
		hints = []string{
			FooterHint("Enter", "confirm"),
			FooterHint("Esc", "cancel"),
			FooterHint("q", "quit"),
		}
	default:
		hints = []string{
			FooterHint("Enter", "edit"),
			FooterHint("←→", "section"),
			FooterHint("↑↓", "navigate"),
			FooterHint("a/d", "patterns"),
			FooterHint("R", "reload"),
			FooterHint("q", "quit"),
		}
	}

	switch {
	case errMsg != "":
		return FitFooterWithStatus(width, errStyle.Render("✗ "+errMsg), hints...)
	case savedMsg != "":
		return FitFooterWithStatus(width, okStyle.Render("✓ "+savedMsg), hints...)
	default:
		return FitFooter(width, hints...)
	}
}

// FormatConfigBool displays a bool for the menu.
func FormatConfigBool(b bool) string {
	return fmt.Sprintf("%v", b)
}

// deviceModalRows bounds the device picker so it always fits on screen.
func deviceModalRows(height int) int {
	const chrome = 12
	n := height - chrome
	switch {
	case height <= 0:
		return 10
	case n < 4:
		return 4
	case n > 18:
		return 18
	default:
		return n
	}
}
