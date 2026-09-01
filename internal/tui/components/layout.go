package components

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"anoted/internal/buildinfo"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// All styles live in theme.go so the light/dark palettes swap in one place.

// TabID identifies the active main screen (switch with 1–3, not Tab).
type TabID int

const (
	TabHome TabID = iota
	TabDoctor
	TabConfig
)

var tabLabels = []string{"Home", "Doctor", "Config"}

// Header renders the app title line, including the running build so a user
// reporting a problem can say which version they are on without digging.
//
// The recording badge lives here rather than in a single screen: anoted must
// make an active recording obvious at all times, and the old indicator only
// existed on Home, so recording while sitting in Config or Doctor showed nothing
// at all.
func Header(subtitle string, recording bool, elapsed time.Duration, pulse bool) string {
	out := headerStyle.Render("anoted") + subtleStyle.Render(" "+buildinfo.Version())
	if subtitle != "" {
		out += subtleStyle.Render(" · " + subtitle)
	}
	if recording {
		// The dot alternates on the 1s duration tick so the badge visibly
		// pulses, and the elapsed clock makes the badge informative rather than
		// merely present — this is the privacy-critical indicator.
		dot := "●"
		if pulse {
			dot = "○"
		}
		out += "  " + recStyle.Render(" "+dot+" "+LabelRecording+" "+formatClock(elapsed)+" ")
	}
	return out
}

// formatClock renders elapsed recording time as m:ss, or h:mm:ss past an hour.
func formatClock(d time.Duration) string {
	d = d.Round(time.Second)
	h := int(d / time.Hour)
	m := int(d/time.Minute) % 60
	s := int(d/time.Second) % 60
	if h > 0 {
		return fmt.Sprintf("%d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%d:%02d", m, s)
}

// TabBar renders the top main navigation tabs (visual only; use 1–3 to switch).
func TabBar(active TabID) string {
	var parts []string
	for i, label := range tabLabels {
		// The marker carries the selection on its own. Styling the active tab
		// only with a background colour left NO_COLOR and monochrome terminals
		// with no way to tell which tab was focused.
		text := fmt.Sprintf("  %d %s", i+1, label)
		if TabID(i) == active {
			parts = append(parts, tabActiveStyle.Render("▸ "+text[2:]))
		} else {
			parts = append(parts, tabInactiveStyle.Render(text))
		}
	}
	return strings.Join(parts, " ")
}

// SubTabBar renders highlighted subsection tabs (e.g. Output / Microphone).
func SubTabBar(labels []string, active int) string {
	var parts []string
	for i, label := range labels {
		text := "[" + label + "]"
		if i == active {
			parts = append(parts, subTabActiveStyle.Render("▸ "+text))
		} else {
			parts = append(parts, subTabInactiveStyle.Render("  "+text))
		}
	}
	return strings.Join(parts, " ")
}

const (
	TwoColumnMinWidth = 80
	// Status needs ~34 columns and the compact equalizer ~40, so 140 left a
	// 120-column terminal — a very common full-screen size — stacking panels
	// vertically and wasting most of its width.
	HomeTopRowMinWidth        = 100 // status | audio side-by-side only when wide enough
	WaveformCompactWidth      = 72  // equalizer uses short layout below this inner width
	SessionsCompactWidth      = 110
	SessionsUltraCompactWidth = 90
	MinPanelWidth             = 28
	panelColumnGap            = 1
)

// PanelLayout computes responsive panel widths for the current terminal.
type PanelLayout struct {
	Width int
}

// NewPanelLayout normalizes width and returns layout helpers for panel rendering.
func NewPanelLayout(width int) PanelLayout {
	if width < MinPanelWidth {
		width = MinPanelWidth
	}
	return PanelLayout{Width: width}
}

// TwoColumn reports whether there is enough room for side-by-side panels.
func (p PanelLayout) TwoColumn() bool {
	return p.Width >= TwoColumnMinWidth
}

// ColumnWidth returns the width for one column in the current layout mode.
func (p PanelLayout) ColumnWidth() int {
	if !p.TwoColumn() {
		return p.Width
	}
	return (p.Width - panelColumnGap) / 2
}

// FullWidth returns the usable content width.
func (p PanelLayout) FullWidth() int {
	return p.Width
}

// JoinColumns places panels side-by-side or stacked vertically based on width.
// The horizontal join is done by hand with the fast width path: lipgloss's
// JoinHorizontal re-measures every line of both blocks with a grapheme scan.
// It does not right-pad the result: every screen flows through PadView, which
// pads each line once.
func (p PanelLayout) JoinColumns(left, right string) string {
	if !p.TwoColumn() {
		return JoinBlocksVertical(left, right)
	}
	ll := strings.Split(left, "\n")
	rl := strings.Split(right, "\n")
	leftW := 0
	for _, l := range ll {
		if w := displayWidth(l); w > leftW {
			leftW = w
		}
	}
	rows := max(len(ll), len(rl))
	var b strings.Builder
	b.Grow(len(left) + len(right) + rows*(panelColumnGap+2))
	for i := 0; i < rows; i++ {
		if i > 0 {
			b.WriteByte('\n')
		}
		if i < len(ll) {
			b.WriteString(padLineWidth(ll[i], leftW))
		} else {
			b.WriteString(padSpaces(leftW))
		}
		b.WriteString(padSpaces(panelColumnGap))
		if i < len(rl) {
			b.WriteString(rl[i])
		}
	}
	return b.String()
}

// JoinBlocksVertical stacks rendered panels with blank lines between them.
func JoinBlocksVertical(blocks ...string) string {
	var parts []string
	for _, block := range blocks {
		block = strings.TrimSpace(block)
		if block != "" {
			parts = append(parts, block)
		}
	}
	return strings.Join(parts, "\n\n")
}

// EqualizeBoxHeights pads shorter boxes so paired columns align cleanly.
func EqualizeBoxHeights(left, right string) (string, string) {
	hLeft := lipgloss.Height(left)
	hRight := lipgloss.Height(right)
	if hLeft < hRight {
		left = lipgloss.Place(lipgloss.Width(left), hRight, lipgloss.Left, lipgloss.Top, left)
	} else if hRight < hLeft {
		right = lipgloss.Place(lipgloss.Width(right), hLeft, lipgloss.Left, lipgloss.Top, right)
	}
	return left, right
}

// Box renders a titled bordered panel.
func Box(title, content string, width int) string {
	return renderBox(title, content, width, borderStyle)
}

// DimBox renders a titled panel with a dim border for unfocused sections.
func DimBox(title, content string, width int) string {
	return renderBox(title, content, width, dimStyle)
}

// renderBox draws the panel by hand: a top border with the title spliced in —
// "╭─ TITLE ────╮" — then side borders around padded content lines. Going
// through lipgloss's border machinery re-measured every line with a full
// grapheme scan; boxes are most of every frame, and displayWidth's fast path
// makes this the difference between ~500µs and ~200µs a frame.
func renderBox(title, content string, width int, lineStyle lipgloss.Style) string {
	if width < MinPanelWidth {
		width = MinPanelWidth
	}
	if content == "" {
		content = subtleStyle.Render("(empty)")
	}
	// lipgloss v2 counts borders inside Width(), and every caller sizes panels
	// on that assumption: the box spans exactly width cells in total.
	inner := width - 4 // borders and one cell of padding on each side
	total := width

	var b strings.Builder
	b.Grow(len(content) + total*3)
	label := strings.ToUpper(title)
	lw := displayWidth(label)
	if lw+5 <= total {
		b.WriteString(lineStyle.Render("╭─"))
		b.WriteString(" ")
		b.WriteString(boxTitleStyle.Render(label))
		b.WriteString(" ")
		b.WriteString(lineStyle.Render(boxDashes(total-5-lw) + "╮"))
	} else {
		b.WriteString(lineStyle.Render("╭" + boxDashes(total-2) + "╮"))
	}
	side := lineStyle.Render("│")
	for line := range strings.SplitSeq(content, "\n") {
		if displayWidth(line) > inner {
			// Long lines wrap, as the lipgloss-rendered box wrapped them.
			for wrapped := range strings.SplitSeq(ansi.Wrap(line, inner, ""), "\n") {
				writeBoxLine(&b, side, wrapped, inner)
			}
			continue
		}
		writeBoxLine(&b, side, line, inner)
	}
	b.WriteString("\n")
	b.WriteString(lineStyle.Render("╰" + boxDashes(total-2) + "╯"))
	return b.String()
}

func writeBoxLine(b *strings.Builder, side, line string, inner int) {
	b.WriteString("\n")
	b.WriteString(side)
	b.WriteString(" ")
	b.WriteString(padLineWidth(line, inner))
	b.WriteString(" ")
	b.WriteString(side)
}

var dashesPool = strings.Repeat("─", 256)

func boxDashes(n int) string {
	if n <= 0 {
		return ""
	}
	// A rune is 3 bytes here, so index by bytes.
	if n*3 <= len(dashesPool) {
		return dashesPool[:n*3]
	}
	return strings.Repeat("─", n)
}

// Badge renders a small status pill with a real background, so DEFAULT /
// RUNNING / N-A read like the REC badge instead of plain colored text.
func Badge(text string, kind string) string {
	switch kind {
	case "ok", "running", "ready":
		return badgeOKStyle.Render(" " + text + " ")
	case "warn", "default":
		return badgeWarnStyle.Render(" " + text + " ")
	case "rec":
		return recStyle.Render(" " + text + " ")
	case "meet":
		return badgeMeetStyle.Render(" " + text + " ")
	default:
		return badgeNeutralStyle.Render(" " + text + " ")
	}
}

// FooterHint renders a keyboard shortcut in the footer.
func FooterHint(key, action string) string {
	return keyStyle.Render(key) + subtleStyle.Render(" "+action)
}

// JoinFooter joins footer hints with separators.
func JoinFooter(hints ...string) string {
	return subtleStyle.Render(strings.Join(hints, "  ·  "))
}

const footerSeparatorWidth = 5 // "  ·  "

// FitFooter joins as many hints as fit on one line, most important first, and
// appends a pointer to the help overlay when it has to drop any.
//
// FooterBar renders through a lipgloss Style with an explicit width, and
// Render wraps rather than truncates. Home's full hint list measures 158 cells,
// so on anything narrower than a 160-column terminal the footer silently became
// two or three rows, pushing content off the bottom of the screen.
func FitFooter(width int, hints ...string) string {
	if width <= 0 || len(hints) == 0 {
		return JoinFooter(hints...)
	}
	helpHint := FooterHint("?", "help")
	helpWidth := displayWidth(helpHint) + footerSeparatorWidth

	fitted := make([]string, 0, len(hints))
	used := 0
	for i, h := range hints {
		w := displayWidth(h)
		if i > 0 {
			w += footerSeparatorWidth
		}
		// Leave room for the "? help" pointer unless this is the last hint and
		// everything fits without it.
		reserve := helpWidth
		if i == len(hints)-1 {
			reserve = 0
		}
		if used+w+reserve > width {
			break
		}
		fitted = append(fitted, h)
		used += w
	}
	if len(fitted) < len(hints) {
		fitted = append(fitted, helpHint)
	}
	return JoinFooter(fitted...)
}

// FooterBar renders a single-line footer bar (htop/lazygit style).
func FooterBar(hints string, width int) string {
	if width > 0 {
		return footerBarStyle.Width(width).Render(hints)
	}
	return footerBarStyle.Render(hints)
}

// FooterWithTrailingStatus places status text flush-right on the footer line.
func FooterWithTrailingStatus(hints, status string, width int) string {
	if status == "" {
		return hints
	}
	if width <= 0 {
		return JoinFooter(hints, status)
	}
	gap := width - displayWidth(hints) - displayWidth(status)
	if gap < 2 {
		return JoinFooter(hints, status)
	}
	return hints + padSpaces(gap) + status
}

// PadView expands content to fill the terminal, avoiding uncleared cells
// after resize (notably on Windows Terminal alt-screen).
func PadView(content string, width, height int) string {
	if width <= 0 {
		return content
	}
	lines := strings.Split(strings.TrimRight(content, "\n"), "\n")
	if len(lines) == 1 && lines[0] == "" {
		lines = nil
	}
	// Keep the last line visible when the content overflows.
	//
	// PadView only ever padded, never truncated, so on a terminal too short for
	// the panels the alternate screen clipped the bottom — taking the footer
	// with it, which is the only place shortcuts are listed. Dropping from the
	// middle keeps the header and the footer, which are the two rows that
	// orient the user.
	if height > 2 && len(lines) > height {
		keepTail := 1 // the footer
		keepHead := height - keepTail - 1
		trimmed := make([]string, 0, height)
		trimmed = append(trimmed, lines[:keepHead]...)
		trimmed = append(trimmed, subtleStyle.Render(overflowNotice(len(lines)-height+1)))
		trimmed = append(trimmed, lines[len(lines)-keepTail:]...)
		lines = trimmed
	}
	out := make([]string, 0, height)
	for _, line := range lines {
		out = append(out, padLineWidth(line, width))
	}
	blank := padSpaces(width)
	for height > 0 && len(out) < height {
		out = append(out, blank)
	}
	return strings.Join(out, "\n")
}

func overflowNotice(hidden int) string {
	return fmt.Sprintf("  … %d more rows — resize the terminal, ? for shortcuts", hidden)
}

func padLineWidth(line string, width int) string {
	gap := width - displayWidth(line)
	if gap <= 0 {
		return line
	}
	return line + padSpaces(gap)
}

// spacesPool backs padSpaces; slicing it is allocation-free.
var spacesPool = strings.Repeat(" ", 512)

func padSpaces(n int) string {
	if n <= 0 {
		return ""
	}
	if n <= len(spacesPool) {
		return spacesPool[:n]
	}
	return strings.Repeat(" ", n)
}

// displayWidth measures a styled line in terminal cells. It short-circuits the
// common case — ASCII text, SGR color sequences and the single-width drawing
// runes this package emits — and falls back to lipgloss.Width (a full ANSI +
// grapheme scan, the hottest call in the frame profile) the moment it sees
// anything it cannot prove is one cell wide.
func displayWidth(s string) int {
	w := 0
	for i := 0; i < len(s); {
		c := s[i]
		switch {
		case c == 0x1b: // ESC: skip a CSI sequence (SGR colors)
			j := i + 1
			if j < len(s) && s[j] == '[' {
				j++
				for j < len(s) {
					b := s[j]
					j++
					if b >= 0x40 && b <= 0x7e { // final byte
						break
					}
				}
				i = j
				continue
			}
			return lipgloss.Width(s) // OSC or bare ESC: bail
		case c >= 0x20 && c < 0x7f: // printable ASCII
			w++
			i++
		case c < 0x20 || c == 0x7f: // other control bytes
			return lipgloss.Width(s)
		default:
			r, size := utf8.DecodeRuneInString(s[i:])
			if !oneCellRune(r) {
				return lipgloss.Width(s)
			}
			w++
			i += size
		}
	}
	return w
}

// oneCellRune reports runes this package knows render one cell wide, matching
// what lipgloss.Width would answer for them. Anything else — CJK, emoji,
// combining marks, ambiguous codepoints not listed — makes displayWidth fall
// back to the full measurement, so unknown runes are never mis-measured.
func oneCellRune(r rune) bool {
	switch {
	case r >= 0x2500 && r <= 0x259f: // box drawing + block elements: ─ │ ╭ █ ░ ▏
		return true
	case r >= 0x25a0 && r <= 0x25ff: // geometric shapes: ● ○ ▸ (ambiguous → narrow, as lipgloss counts them)
		// Except U+25FD/U+25FE (medium small squares): East-Asian Wide, the
		// only two runes in this block lipgloss measures as 2 cells.
		return r != 0x25fd && r != 0x25fe
	case r >= 0x2800 && r <= 0x28ff: // braille spinner frames
		return true
	}
	switch r {
	case '·', '…', '—', '↑', '↓', '✓', '✗':
		return true
	}
	return false
}

// clampStyledWidth shortens a styled string to fit a panel without wrapping.
func clampStyledWidth(s string, maxW int) string {
	if maxW <= 0 || displayWidth(s) <= maxW {
		return s
	}
	return ansi.Truncate(s, maxW, "…")
}

// LogLineWidth bounds a single line in the scrolling log panes.
const LogLineWidth = 120

// ClampLogLine trims one line of subprocess output to fit a log pane.
//
// These panes carry transcript previews and pip output, so the naive
// line[:119] cut a multibyte character in half and the terminal drew a
// replacement glyph. The same mistake had already been fixed twice elsewhere
// in the codebase without being applied here.
func ClampLogLine(line string) string {
	return clampStyledWidth(line, LogLineWidth)
}

// padCell clamps s to w terminal cells and right-pads it to exactly w.
//
// Table rows must not be built with fmt's %-Ns padding: several cells carry SGR
// escapes, and fmt counts bytes. A seven-cell coloured status took ~19 bytes, so
// its "28-wide" column rendered about 16 cells and every column to its right
// drifted out of line with the header — visibly worse while a progress bar was
// animating and the escape length changed frame to frame.
func padCell(s string, w int) string {
	if w <= 0 {
		return s
	}
	s = clampStyledWidth(s, w)
	if gap := w - displayWidth(s); gap > 0 {
		s += padSpaces(gap)
	}
	return s
}

// FloatCenter overlays content centered on a dimmed background.
func FloatCenter(background, overlay string, width, height int) string {
	if strings.TrimSpace(overlay) == "" {
		return background
	}

	bgLines := strings.Split(strings.TrimRight(background, "\n"), "\n")
	olLines := strings.Split(strings.TrimRight(overlay, "\n"), "\n")

	modalH := len(olLines)
	modalW := 0
	for _, line := range olLines {
		if w := lipgloss.Width(line); w > modalW {
			modalW = w
		}
	}

	bgW, bgH := 0, len(bgLines)
	for _, line := range bgLines {
		if w := lipgloss.Width(line); w > bgW {
			bgW = w
		}
	}

	if width <= 0 {
		width = modalW
	}
	if bgW > width {
		width = bgW
	}
	if height <= 0 {
		height = bgH
	}
	if bgH > height {
		height = bgH
	}

	startY := (height - modalH) / 2
	if startY < 0 {
		startY = 0
	}
	startX := (width - modalW) / 2
	if startX < 0 {
		startX = 0
	}
	if startY+modalH > height {
		height = startY + modalH
	}
	if startX+modalW > width {
		width = startX + modalW
	}

	// Strip the background's own SGR codes before dimming. Style.Render just
	// wraps the string in one escape pair, so the first reset inside the
	// background restored its original colour and only the unstyled prefix of
	// each line actually dimmed — modals ended up floating over a
	// full-brightness screen with no visual focus.
	dimmedLines := make([]string, height)
	for y := 0; y < height; y++ {
		if y < len(bgLines) {
			dimmedLines[y] = dimStyle.Render(ansi.Strip(bgLines[y]))
		} else {
			dimmedLines[y] = ""
		}
	}

	bgLayer := lipgloss.NewLayer(strings.Join(dimmedLines, "\n"))
	olLayer := lipgloss.NewLayer(overlay).X(startX).Y(startY)
	return lipgloss.NewCompositor(bgLayer, olLayer).Render()
}

// PickerModal renders a compact floating picker dialog.
func PickerModal(title string, body string, maxWidth int) string {
	lines := strings.Split(body, "\n")
	modalW := lipgloss.Width(boxTitleStyle.Render(strings.ToUpper(title))) + 4
	for _, line := range lines {
		if w := lipgloss.Width(line); w+4 > modalW {
			modalW = w + 4
		}
	}
	if modalW < 28 {
		modalW = 28
	}
	if maxWidth > 0 && modalW > maxWidth {
		modalW = maxWidth
	}
	inner := boxTitleStyle.Render(strings.ToUpper(title)) + "\n" + body
	return modalBoxStyle.Width(modalW).Render(inner)
}
