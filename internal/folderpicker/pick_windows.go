//go:build windows

package folderpicker

import (
	"context"
	"fmt"
)

func available() bool {
	return true
}

func pick(ctx context.Context, startDir string) (string, bool, error) {
	startDir = resolveStartDir(startDir)
	path, canceled, err := pickWindowsDialog(ctx, "powershell.exe", startDir)
	if err != nil {
		return "", false, fmt.Errorf("folder picker: %w", err)
	}
	return path, canceled, nil
}
