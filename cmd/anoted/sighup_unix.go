//go:build !windows

package main

import (
	"os"
	"os/signal"
	"syscall"

	"anoted/internal/tui"
	tea "charm.land/bubbletea/v2"
)

// forwardHangup routes the terminal dying to the clean quit path. The kernel
// sends SIGHUP to the foreground process group when the pty goes away — the
// window was closed, the terminal emulator killed. Bubble Tea handles SIGINT
// and SIGTERM but not SIGHUP, and the default disposition terminates without
// running defers, so the instance lock outlived the window: every later launch
// bounced off "already running" with no window to show the error in. The quit
// message goes through Update, whose performQuit stops any active recording
// before the defers release the lock.
func forwardHangup(p *tea.Program) {
	hangup := make(chan os.Signal, 1)
	signal.Notify(hangup, syscall.SIGHUP)
	go func() {
		<-hangup
		p.Send(tui.TrayQuitMsg{})
	}()
}
