//go:build !windows

package transcribe

import (
	"fmt"
	"io"
	"os/exec"
)

var unixPythonCandidates = []string{
	"python3",
	"python",
	"/usr/bin/python3",
	"/usr/bin/python",
	"/usr/local/bin/python3",
}

func discoverPython() string {
	for _, name := range unixPythonCandidates {
		path, err := exec.LookPath(name)
		if err != nil {
			continue
		}
		if verifyPython(path) {
			return path
		}
	}
	return ""
}

func ensurePythonInstall(out io.Writer, autoInstall bool) (string, error) {
	if py := discoverPython(); py != "" {
		return py, nil
	}
	_ = out
	if autoInstall {
		return "", fmt.Errorf("python not found — %s", PythonInstallHint())
	}
	return "", fmt.Errorf("python not found — %s", PythonInstallHint())
}
