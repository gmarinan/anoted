//go:build !linux

package doctor

import "anoted/internal/platform"

func detectionChecks(_ platform.Info, _ config.Config) []Check { return nil }
