//go:build linux

package tray

import (
	"os"
	"testing"
)

func TestLinuxBridgeDetailMissingSnixembed(t *testing.T) {
	if os.Getenv("SNIXEMBED_INSTALLED") == "1" {
		t.Skip("snixembed present")
	}
	detail := LinuxBridgeDetail()
	if detail == "" {
		t.Fatal("expected detail")
	}
}
