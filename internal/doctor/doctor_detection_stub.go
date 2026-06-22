//go:build !linux && !windows

package doctor

import (
	"anoted/internal/config"
	"anoted/internal/platform"
)

func detectionChecks(_ platform.Info, _ config.Config) []Check { return nil }
