//go:build linux || windows

package folderpicker

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
)

func pickWindowsDialog(ctx context.Context, powershell, startDir string) (path string, canceled bool, err error) {
	script := windowsFolderScript(startDir)
	cmd := exec.CommandContext(ctx, powershell,
		"-STA", "-NoProfile", "-NonInteractive",
		"-Command", script,
	)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if stdout.Len() == 0 {
			return "", true, nil
		}
		detail := strings.TrimSpace(stderr.String())
		if detail != "" {
			return "", false, fmt.Errorf("powershell folder dialog: %s", detail)
		}
		return "", false, err
	}
	path = strings.TrimSpace(stdout.String())
	if path == "" {
		return "", true, nil
	}
	return path, false, nil
}

func windowsFolderScript(startDir string) string {
	startDir = strings.ReplaceAll(startDir, "'", "''")
	return fmt.Sprintf(`
Add-Type -AssemblyName System.Windows.Forms
$d = New-Object System.Windows.Forms.FolderBrowserDialog
$d.Description = 'Select folder'
$d.UseDescriptionForTitle = $true
if ('%s' -ne '' -and (Test-Path -LiteralPath '%s')) {
  $d.SelectedPath = '%s'
}
if ($d.ShowDialog() -eq [System.Windows.Forms.DialogResult]::OK) {
  Write-Output $d.SelectedPath
}
`, startDir, startDir, startDir)
}
