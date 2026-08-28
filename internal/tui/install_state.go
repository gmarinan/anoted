package tui

import (
	"context"
	"strings"

	"anoted/internal/tui/components"
)

// installLogMax bounds a single install log buffer.
const installLogMax = 40

// installState is the state of one long-running installation.
//
// The Whisper and GPU installers were two copies of the same five fields and
// the same four methods — append with truncation, a scroll offset, a max-scroll
// calculation and a window — differing only in their identifiers. One of the
// copies then drifted: the GPU pane had scrolling and the Whisper pane did not,
// which is why the Whisper log grew until it pushed the footer off the screen.
// Sharing the type means a fix lands on both.
type installState struct {
	Active bool
	Log    []string
	Err    string
	Scroll int
	Cancel context.CancelFunc
}

// begin resets the state for a new run and stores its cancel function.
func (s *installState) begin(seed string, cancel context.CancelFunc) {
	s.Active = true
	s.Log = []string{seed}
	s.Err = ""
	s.Scroll = 0
	s.Cancel = cancel
}

// finish clears the active flag and releases the context.
func (s *installState) finish() {
	s.Active = false
	s.cancel()
}

// cancel invokes and clears the cancel function. Dropping the reference without
// calling it leaks the context — a mistake this codebase made three times.
func (s *installState) cancel() {
	if s.Cancel != nil {
		s.Cancel()
		s.Cancel = nil
	}
}

func (s *installState) appendLine(line string) {
	line = strings.TrimSpace(line)
	if line == "" {
		return
	}
	s.Log = append(s.Log, components.ClampLogLine(line))
	if len(s.Log) > installLogMax {
		s.Log = s.Log[len(s.Log)-installLogMax:]
	}
}

// visible returns the window of log lines to draw.
func (s installState) visible(viewHeight int) []string {
	if viewHeight < 1 {
		viewHeight = 6
	}
	if len(s.Log) <= viewHeight {
		return s.Log
	}
	start := s.Scroll
	if maxStart := s.maxScroll(viewHeight); start > maxStart {
		start = maxStart
	}
	if start < 0 {
		start = 0
	}
	return s.Log[start : start+viewHeight]
}

func (s installState) maxScroll(viewHeight int) int {
	if n := len(s.Log) - viewHeight; n > 0 {
		return n
	}
	return 0
}

// scrollBy moves the window and clamps it, so both panes behave identically.
func (s *installState) scrollBy(delta, viewHeight int) {
	s.Scroll = clampScroll(s.Scroll+delta, s.maxScroll(viewHeight))
}

// hasContent reports whether the pane should be drawn at all.
func (s installState) hasContent() bool {
	return s.Active || s.Err != "" || len(s.Log) > 0
}
