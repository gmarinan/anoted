//go:build linux

package tray

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/godbus/dbus/v5"
)

const statusNotifierWatcher = "org.kde.StatusNotifierWatcher"

var bridgeOnce sync.Once
var bridgeErr error

// EnsureLinuxBridge starts snixembed when needed so SNI tray icons appear in
// i3bar and other XEmbed-only status bars.
func EnsureLinuxBridge() error {
	bridgeOnce.Do(func() {
		bridgeErr = ensureLinuxBridge()
	})
	return bridgeErr
}

// LinuxBridgeDetail returns a human-readable tray host status for doctor.
func LinuxBridgeDetail() string {
	if hasStatusNotifierWatcher() {
		if bridgeProcessRunning() {
			return "StatusNotifierWatcher active (snixembed)"
		}
		return "StatusNotifierWatcher active"
	}
	if _, err := exec.LookPath("snixembed"); err != nil {
		return "missing snixembed — required for i3/i3bar (pacman -S snixembed)"
	}
	if bridgeProcessRunning() {
		return "snixembed running but StatusNotifierWatcher not found"
	}
	return "snixembed not running — start with: snixembed --fork"
}

func ensureLinuxBridge() error {
	if hasStatusNotifierWatcher() {
		return nil
	}
	if !x11Session() {
		return nil
	}
	bin, err := exec.LookPath("snixembed")
	if err != nil {
		return fmt.Errorf("tray bridge: install snixembed for i3/sway bars (sudo pacman -S snixembed)")
	}
	if bridgeProcessRunning() {
		return waitForWatcher()
	}
	cmd := exec.Command(bin, "--fork")
	cmd.Env = append(os.Environ(), "GDK_BACKEND=x11")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("tray bridge: snixembed --fork: %w (%s)", err, strings.TrimSpace(string(out)))
	}
	return waitForWatcher()
}

func waitForWatcher() error {
	for i := 0; i < 20; i++ {
		if hasStatusNotifierWatcher() {
			return nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	return fmt.Errorf("tray bridge: StatusNotifierWatcher did not start — run: GDK_BACKEND=x11 snixembed --fork")
}

func x11Session() bool {
	if strings.TrimSpace(os.Getenv("WAYLAND_DISPLAY")) != "" &&
		strings.TrimSpace(os.Getenv("DISPLAY")) == "" {
		return false
	}
	return strings.TrimSpace(os.Getenv("DISPLAY")) != ""
}

func bridgeProcessRunning() bool {
	err := exec.Command("pgrep", "-x", "snixembed").Run()
	return err == nil
}

func hasStatusNotifierWatcher() bool {
	conn, err := dbus.SessionBus()
	if err != nil {
		return false
	}
	var has bool
	err = conn.BusObject().Call("org.freedesktop.DBus.NameHasOwner", 0, statusNotifierWatcher).Store(&has)
	return err == nil && has
}
