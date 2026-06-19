//go:build !linux

package doctor

import "meetctl/internal/platform"

func detectionChecks(_ platform.Info, _ config.Config) []Check { return nil }
