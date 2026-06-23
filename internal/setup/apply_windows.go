//go:build windows

package setup

import (
	"context"
	"fmt"
	"os/exec"
)

func verifyWindowsTitles(ctx context.Context) error {
	script := `Get-Process | Where-Object { $_.MainWindowTitle } | Select-Object -First 1 | ForEach-Object { $_.MainWindowTitle }`
	out, err := exec.CommandContext(ctx, "powershell", "-NoProfile", "-Command", script).Output()
	if err != nil {
		return fmt.Errorf("powershell: %w", err)
	}
	if len(out) == 0 {
		return fmt.Errorf("no window titles returned (open a browser window to verify)")
	}
	return nil
}
