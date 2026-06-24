//go:build !linux && !windows

package folderpicker

import "context"

func available() bool {
	return false
}

func pick(ctx context.Context, startDir string) (string, bool, error) {
	return "", false, ErrUnavailable
}
