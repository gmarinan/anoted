//go:build linux

package folderpicker

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func available() bool {
	return firstLinuxPicker() != ""
}

func pick(ctx context.Context, startDir string) (string, bool, error) {
	startDir = resolveStartDir(startDir)

	if bin := firstLinuxPicker(); bin != "" {
		path, canceled, err := pickLinux(ctx, bin, startDir)
		if err == nil || canceled {
			return path, canceled, err
		}
	}

	if ps := wslPowerShell(); ps != "" {
		return pickWindowsDialog(ctx, ps, startDir)
	}

	return "", false, fmt.Errorf("%w (install zenity, kdialog, or yad)", ErrUnavailable)
}

type linuxPicker struct {
	bin  string
	args func(startDir string) []string
}

var linuxPickers = []linuxPicker{
	{
		bin: "zenity",
		args: func(startDir string) []string {
			args := []string{"--file-selection", "--directory", "--title=Select folder"}
			if startDir != "" {
				args = append(args, "--filename="+startDir+"/")
			}
			return args
		},
	},
	{
		bin: "kdialog",
		args: func(startDir string) []string {
			args := []string{"--getexistingdirectory", "--title", "Select folder"}
			if startDir != "" {
				args = append(args, startDir)
			} else {
				args = append(args, ".")
			}
			return args
		},
	},
	{
		bin: "yad",
		args: func(startDir string) []string {
			args := []string{"--file", "--directory", "--title=Select folder"}
			if startDir != "" {
				args = append(args, "--filename="+startDir+"/")
			}
			return args
		},
	},
}

func firstLinuxPicker() string {
	for _, p := range linuxPickers {
		if _, err := exec.LookPath(p.bin); err == nil {
			return p.bin
		}
	}
	return ""
}

func pickLinux(ctx context.Context, bin, startDir string) (string, bool, error) {
	var spec linuxPicker
	for _, p := range linuxPickers {
		if p.bin == bin {
			spec = p
			break
		}
	}
	if spec.bin == "" {
		return "", false, fmt.Errorf("unknown picker %q", bin)
	}

	cmd := exec.CommandContext(ctx, spec.bin, spec.args(startDir)...)
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return "", true, nil
		}
		return "", false, fmt.Errorf("%s: %w", spec.bin, err)
	}
	path := strings.TrimSpace(string(out))
	if path == "" {
		return "", true, nil
	}
	return path, false, nil
}

func wslPowerShell() string {
	if !isWSL() {
		return ""
	}
	for _, p := range []string{
		"/mnt/c/Windows/System32/WindowsPowerShell/v1.0/powershell.exe",
		"/mnt/c/Windows/Sysnative/WindowsPowerShell/v1.0/powershell.exe",
	} {
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p
		}
	}
	return ""
}

func isWSL() bool {
	data, err := os.ReadFile("/proc/version")
	if err != nil {
		return false
	}
	lower := strings.ToLower(string(data))
	return strings.Contains(lower, "microsoft") || strings.Contains(lower, "wsl")
}
