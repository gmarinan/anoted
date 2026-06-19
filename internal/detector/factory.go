package detector

import (
	"meetctl/internal/config"
	"meetctl/internal/platform"
	"time"
)

// New creates the best available detector for the current platform.
func New(cfg config.Config, plat platform.Info, useMock bool) Detector {
	detCfg := Config{
		PollInterval: time.Duration(cfg.Detection.PollIntervalMS) * time.Millisecond,
		Providers:    providerPatterns(cfg),
		Mode:         cfg.Detection.Mode,
		WindowTool:   cfg.Detection.WindowTool,
	}
	if useMock {
		return NewMockDetector()
	}
	return newPlatformDetector(detCfg, plat)
}

func providerPatterns(cfg config.Config) map[string][]string {
	out := make(map[string][]string, len(cfg.Detection.Providers))
	for name, p := range cfg.Detection.Providers {
		out[name] = append([]string(nil), p.Patterns...)
	}
	return out
}
