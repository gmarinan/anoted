//go:build windows

package setup

import "os/exec"

func hasCmd(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}
