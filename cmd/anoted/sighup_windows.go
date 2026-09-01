//go:build windows

package main

import (
	tea "charm.land/bubbletea/v2"
)

// Windows delivers no SIGHUP; a closing console terminates the process
// through its own path.
func forwardHangup(*tea.Program) {}
