package doctor

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"anoted/internal/config"
	"anoted/internal/platform"
	"anoted/internal/recorder"
	"anoted/internal/transcribe"
)

// Check describes a single diagnostic result.
type Check struct {
	Name   string
	Status string // ok, warn, fail
	Detail string
}

// Report is the full doctor output.
type Report struct {
	Platform platform.Info
	Checks   []Check
}

// Run executes system diagnostics.
func Run(cfg config.Config) Report {
	plat := platform.Detect()
	rep := Report{Platform: plat}

	rep.Checks = append(rep.Checks,
		Check{Name: "operating_system", Status: "ok", Detail: plat.Name()},
		wslCheck(plat),
		outputDirCheck(cfg),
	)

	rep.Checks = append(rep.Checks, optionalToolChecks(cfg)...)

	if runtime.GOOS == "windows" {
		// Enumerate capture sessions before WASAPI/malgo init to avoid COM conflicts.
		rep.Checks = append(rep.Checks, detectionChecks(plat, cfg)...)
		rep.Checks = append(rep.Checks, audioDeviceChecks(cfg)...)
	} else {
		rep.Checks = append(rep.Checks, audioDeviceChecks(cfg)...)
		rep.Checks = append(rep.Checks, detectionChecks(plat, cfg)...)
	}

	rec := recorder.New(cfg, plat, false)
	rep.Checks = append(rep.Checks, Check{
		Name:   "recorder_backend",
		Status: "ok",
		Detail: rec.Name(),
	})

	if plat.IsWSL2 {
		rep.Checks = append(rep.Checks, helperCheck())
	}

	for _, c := range transcribe.DoctorChecks(cfg) {
		rep.Checks = append(rep.Checks, Check{Name: c.Name, Status: c.Status, Detail: c.Detail})
	}

	rep.Checks = append(rep.Checks, desktopCheck(cfg))
	rep.Checks = append(rep.Checks, autostartCheck())
	rep.Checks = append(rep.Checks, trayCheck(cfg))

	return rep
}

func wslCheck(plat platform.Info) Check {
	if plat.IsWSL2 {
		return Check{Name: "wsl2", Status: "warn", Detail: "running in WSL2; use windows-recorder.exe for real Windows audio"}
	}
	return Check{Name: "wsl2", Status: "ok", Detail: "not WSL2"}
}

func outputDirCheck(cfg config.Config) Check {
	dir, err := cfg.ResolvedOutputDir()
	if err != nil {
		return Check{Name: "output_dir", Status: "fail", Detail: err.Error()}
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return Check{Name: "output_dir", Status: "fail", Detail: err.Error()}
	}
	return Check{Name: "output_dir", Status: "ok", Detail: dir}
}

func commandCheck(name string) Check {
	path, err := exec.LookPath(name)
	if err != nil {
		return Check{Name: name, Status: "warn", Detail: "not found in PATH"}
	}
	return Check{Name: name, Status: "ok", Detail: path}
}

func helperCheck() Check {
	candidates := []string{
		"/mnt/c/Program Files/anoted/windows-recorder.exe",
		"windows-recorder.exe",
	}
	for _, c := range candidates {
		if _, err := exec.LookPath(c); err == nil {
			return Check{Name: "windows_helper", Status: "ok", Detail: c}
		}
	}
	return Check{Name: "windows_helper", Status: "warn", Detail: "windows-recorder.exe not found"}
}

// Format renders the report as human-readable text.
func Format(rep Report) string {
	var b strings.Builder
	fmt.Fprintf(&b, "anoted doctor\n")
	fmt.Fprintf(&b, "Platform: %s (session: %s)\n\n", rep.Platform.Name(), rep.Platform.Session)
	for _, c := range rep.Checks {
		fmt.Fprintf(&b, "[%s] %s: %s\n", c.Status, c.Name, c.Detail)
	}
	return b.String()
}
