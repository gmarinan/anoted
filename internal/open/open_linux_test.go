//go:build linux

package open

import (
	"testing"
)

func TestIsDiskUsageHandler(t *testing.T) {
	if !isDiskUsageHandler("org.gnome.baobab.desktop") {
		t.Fatal("baobab should be detected")
	}
	if isDiskUsageHandler("org.kde.dolphin.desktop") {
		t.Fatal("dolphin should not be disk usage handler")
	}
}

func TestAutoFolderCommandPrefersFM(t *testing.T) {
	cmd, args, err := autoFolderCommand("/tmp/meetctl-test")
	if err != nil {
		t.Skip(err)
	}
	if len(args) != 1 || args[0] != "/tmp/meetctl-test" {
		t.Fatalf("unexpected args: %v", args)
	}
	if cmd == "" {
		t.Fatal("expected command")
	}
}
