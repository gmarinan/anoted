//go:build linux

package detector

import (
	"context"
	"os/exec"
	"strings"

	"meetctl/internal/platform"
)

func newPlatformDetector(cfg Config, plat platform.Info) Detector {
	return &linuxDetector{cfg: cfg, plat: plat}
}

type linuxDetector struct {
	cfg  Config
	plat platform.Info
}

func (d *linuxDetector) Name() string { return "linux" }

func listProcesses(ctx context.Context) ([]string, error) {
	out, err := exec.CommandContext(ctx, "ps", "-eo", "comm=").Output()
	if err != nil {
		return nil, err
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	var names []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if i := strings.LastIndex(line, "/"); i >= 0 {
			line = line[i+1:]
		}
		names = append(names, strings.TrimSuffix(line, ".exe"))
	}
	return names, nil
}
