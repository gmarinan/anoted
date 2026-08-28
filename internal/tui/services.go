package tui

import (
	"context"

	"anoted/internal/autostart"
	"anoted/internal/config"
	"anoted/internal/folderpicker"
	"anoted/internal/open"
)

// The TUI reaches three build-tagged packages — internal/open,
// internal/autostart and internal/folderpicker — as package globals. There is
// no OS-specific logic inside internal/tui, so the letter of the architecture rule
// held, but the injection half did not: nothing could substitute a file manager
// or a folder dialog in a test, which is a large part of why this package sits
// at 12% coverage.
//
// These interfaces are deliberately small, matching the audio.Provider and
// level.Monitor pattern already used in Deps.

// Opener launches folders and files in the user's desktop environment.
type Opener interface {
	Open(path string, cfg config.DesktopConfig, kind open.Kind) error
	CurrentOpenerID(cfg config.DesktopConfig) string
	OpenerOptions(cfg config.DesktopConfig) []open.OpenerOption
	Detected(cfg config.DesktopConfig, kind open.Kind) string
}

// Autostart manages the launch-at-login entry.
type Autostart interface {
	Available() bool
	Enabled() bool
	EntryFromConfig(cfg config.Config) (autostart.Entry, error)
	Enable(entry autostart.Entry) error
	Disable() error
}

// FolderPicker shows the platform's native directory chooser.
type FolderPicker interface {
	Pick(ctx context.Context, startDir string) (path string, canceled bool, err error)
}

// systemOpener is the production Opener, backed by internal/open.
type systemOpener struct{}

func (systemOpener) Open(path string, cfg config.DesktopConfig, kind open.Kind) error {
	return open.Open(path, cfg, kind)
}
func (systemOpener) CurrentOpenerID(cfg config.DesktopConfig) string {
	return open.CurrentOpenerID(cfg)
}
func (systemOpener) OpenerOptions(cfg config.DesktopConfig) []open.OpenerOption {
	return open.OpenerOptions(cfg)
}
func (systemOpener) Detected(cfg config.DesktopConfig, kind open.Kind) string {
	return open.Detected(cfg, kind)
}

// systemAutostart is the production Autostart, backed by internal/autostart.
type systemAutostart struct{}

func (systemAutostart) Available() bool { return autostart.Available() }
func (systemAutostart) Enabled() bool   { return autostart.Enabled() }
func (systemAutostart) EntryFromConfig(cfg config.Config) (autostart.Entry, error) {
	return autostart.EntryFromConfig(cfg)
}
func (systemAutostart) Enable(entry autostart.Entry) error { return autostart.Enable(entry) }
func (systemAutostart) Disable() error                     { return autostart.Disable() }

// systemFolderPicker is the production FolderPicker.
type systemFolderPicker struct{}

func (systemFolderPicker) Pick(ctx context.Context, startDir string) (string, bool, error) {
	return folderpicker.Pick(ctx, startDir)
}

// defaultServices fills in any service the caller left nil, so a Deps built by
// a test only has to provide what that test cares about.
func (d Deps) withDefaults() Deps {
	if d.Opener == nil {
		d.Opener = systemOpener{}
	}
	if d.Autostart == nil {
		d.Autostart = systemAutostart{}
	}
	if d.FolderPicker == nil {
		d.FolderPicker = systemFolderPicker{}
	}
	return d
}
