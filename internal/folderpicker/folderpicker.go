package folderpicker

import (
	"context"
	"errors"
)

// ErrUnavailable means no native folder picker is installed on this host.
var ErrUnavailable = errors.New("no native folder picker available")

// Pick opens a native folder selection dialog.
// startDir is an optional initial directory (empty uses a sensible default).
// Returns canceled=true when the user dismisses the dialog without choosing.
func Pick(ctx context.Context, startDir string) (path string, canceled bool, err error) {
	return pick(ctx, startDir)
}

// Available reports whether Pick can open a native folder dialog.
func Available() bool {
	return available()
}
