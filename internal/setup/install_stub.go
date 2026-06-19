//go:build !linux

package setup

import (
	"fmt"
	"io"
)

func installTool(_ io.Writer, tool string) error {
	return fmt.Errorf("automatic install of %s is only supported on Linux", tool)
}

func installCommand(_ string) ([]string, bool) {
	return nil, false
}
