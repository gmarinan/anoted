//go:build windows

package transcribe

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const wingetPythonID = "Python.Python.3.12"

func discoverPython() string {
	if py, ok := resolvePyLauncher(); ok {
		return py
	}
	for _, candidate := range windowsPythonCandidates() {
		if py, ok := tryPythonCandidate(candidate); ok {
			return py
		}
	}
	return ""
}

func windowsPythonCandidates() []string {
	var candidates []string
	for _, dir := range windowsPythonInstallDirs() {
		candidates = append(candidates, filepath.Join(dir, "python.exe"))
	}
	candidates = append(candidates,
		"python3",
		"python",
	)
	return candidates
}

func windowsPythonInstallDirs() []string {
	var dirs []string
	if lad := os.Getenv("LOCALAPPDATA"); lad != "" {
		dirs = append(dirs, globPythonDirs(filepath.Join(lad, "Programs", "Python", "Python3*"))...)
	}
	if pf := os.Getenv("ProgramFiles"); pf != "" {
		dirs = append(dirs, globPythonDirs(filepath.Join(pf, "Python3*"))...)
	}
	if pf86 := os.Getenv("ProgramFiles(x86)"); pf86 != "" {
		dirs = append(dirs, globPythonDirs(filepath.Join(pf86, "Python3*"))...)
	}
	return dirs
}

func globPythonDirs(pattern string) []string {
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil
	}
	var dirs []string
	for _, m := range matches {
		if info, err := os.Stat(m); err == nil && info.IsDir() {
			dirs = append(dirs, m)
		}
	}
	return dirs
}

func resolvePyLauncher() (string, bool) {
	if _, err := exec.LookPath("py"); err != nil {
		return "", false
	}
	cmd := exec.Command("py", "-3", "-c", "import sys; print(sys.executable)")
	out, err := cmd.Output()
	if err != nil {
		return "", false
	}
	path := strings.TrimSpace(string(out))
	if path == "" || !verifyPython(path) {
		return "", false
	}
	return path, true
}

func tryPythonCandidate(name string) (string, bool) {
	var path string
	if filepath.IsAbs(name) {
		if _, err := os.Stat(name); err != nil {
			return "", false
		}
		path = name
	} else {
		p, err := exec.LookPath(name)
		if err != nil {
			return "", false
		}
		path = p
	}
	if isStorePythonStub(path) {
		return "", false
	}
	if !verifyPython(path) {
		return "", false
	}
	return path, true
}

func ensurePythonInstall(out io.Writer, autoInstall bool) (string, error) {
	if py := discoverPython(); py != "" {
		return py, nil
	}
	if !autoInstall {
		return "", fmt.Errorf("python not found — %s", PythonInstallHint())
	}
	if !hasCmd("winget") {
		return "", fmt.Errorf("python not found and winget unavailable — %s", PythonInstallHint())
	}

	fmt.Fprintln(out, "  Installing Python 3.12 via winget (may take a few minutes)…")
	cmd := exec.Command("winget", "install", "--id", wingetPythonID, "-e",
		"--disable-interactivity",
		"--accept-package-agreements",
		"--accept-source-agreements",
	)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("winget install python: %w — try: %s", err, PythonInstallHint())
	}

	if py := discoverPython(); py != "" {
		fmt.Fprintf(out, "  ✓ Python ready: %s\n", py)
		return py, nil
	}
	return "", fmt.Errorf("python installed but not found in standard paths — restart the terminal or run: %s", PythonInstallHint())
}
