//go:build !linux

package doctor

import "anoted/internal/config"

func audioDeviceChecks(_ config.Config) []Check { return nil }
