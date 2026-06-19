//go:build linux

package setup

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

func installTool(out io.Writer, tool string) error {
	cmd, ok := installCommand(tool)
	if !ok {
		return fmt.Errorf("unknown package manager; install %s manually", tool)
	}
	fmt.Fprintf(out, "\n  Running: %s\n\n", joinCmd(cmd))
	c := exec.Command(cmd[0], cmd[1:]...)
	c.Stdin = os.Stdin
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	if err := c.Run(); err != nil {
		return fmt.Errorf("install %s: %w", tool, err)
	}
	if !hasTool(tool) {
		return fmt.Errorf("%s not found after install", tool)
	}
	return nil
}

func installCommand(tool string) ([]string, bool) {
	pkg := tool
	switch {
	case hasCmd("pacman"):
		return []string{"sudo", "pacman", "-S", "--needed", "--noconfirm", pkg}, true
	case hasCmd("apt-get"):
		return []string{"sudo", "apt-get", "install", "-y", pkg}, true
	case hasCmd("dnf"):
		return []string{"sudo", "dnf", "install", "-y", pkg}, true
	case hasCmd("zypper"):
		return []string{"sudo", "zypper", "install", "-y", pkg}, true
	default:
		return nil, false
	}
}

func hasCmd(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

func joinCmd(parts []string) string {
	return strings.Join(parts, " ")
}
