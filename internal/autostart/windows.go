//go:build windows

package autostart

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const startupName = "anoted.cmd"

func available() bool {
	_, err := startupDir()
	return err == nil
}

func enabled() bool {
	path, err := entryPath()
	if err != nil {
		return false
	}
	_, err = os.Stat(path)
	return err == nil
}

func entryPath() (string, error) {
	dir, err := startupDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, startupName), nil
}

func startupDir() (string, error) {
	appData := strings.TrimSpace(os.Getenv("APPDATA"))
	if appData == "" {
		return "", fmt.Errorf("APPDATA is not set")
	}
	return filepath.Join(appData, "Microsoft", "Windows", "Start Menu", "Programs", "Startup"), nil
}

func enable(entry Entry) error {
	dir, err := startupDir()
	if err != nil {
		return err
	}
	path := filepath.Join(dir, startupName)
	content := renderStartupCmd(entry)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return fmt.Errorf("write startup script: %w", err)
	}
	return nil
}

func disable() error {
	path, err := entryPath()
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove startup script: %w", err)
	}
	return nil
}

func renderStartupCmd(entry Entry) string {
	exe := strings.ReplaceAll(entry.Exec, `"`, `""`)
	var b strings.Builder
	b.WriteString("@echo off\r\n")
	b.WriteString(`start "anoted" "` + exe + `"`)
	for _, arg := range entry.Args {
		b.WriteString(" ")
		if strings.ContainsAny(arg, " \t") {
			b.WriteString(`"` + strings.ReplaceAll(arg, `"`, `""`) + `"`)
		} else {
			b.WriteString(arg)
		}
	}
	b.WriteString("\r\n")
	return b.String()
}
