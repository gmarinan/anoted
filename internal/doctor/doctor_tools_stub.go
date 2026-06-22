//go:build !linux && !windows

package doctor

import "anoted/internal/config"

func optionalToolChecks(_ config.Config) []Check {
	return nil
}
