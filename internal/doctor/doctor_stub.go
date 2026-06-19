//go:build !linux

package doctor

import "meetctl/internal/config"

func audioDeviceChecks(_ config.Config) []Check { return nil }
