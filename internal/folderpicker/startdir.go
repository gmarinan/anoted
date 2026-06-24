package folderpicker

import (
	"os"
	"strings"
)

func resolveStartDir(startDir string) string {
	startDir = strings.TrimSpace(startDir)
	if startDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		return home
	}
	if st, err := os.Stat(startDir); err != nil || !st.IsDir() {
		home, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		return home
	}
	return startDir
}
