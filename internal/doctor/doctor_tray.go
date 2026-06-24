package doctor

import (
	"os"
	"os/exec"
	"runtime"

	"anoted/internal/config"
	"anoted/internal/tray"
)

func trayCheck(cfg config.Config) Check {
	if !cfg.Privacy.TrayIndicator {
		return Check{Name: "tray_indicator", Status: "ok", Detail: "disabled in config"}
	}
	if runtime.GOOS == "linux" {
		if os.Getenv("DBUS_SESSION_BUS_ADDRESS") == "" {
			return Check{Name: "tray_indicator", Status: "warn", Detail: "DBUS_SESSION_BUS_ADDRESS not set — tray may not appear"}
		}
		bridge := tray.LinuxBridgeDetail()
		if !hasStatusNotifierWatcherForDoctor() {
			if _, err := exec.LookPath("snixembed"); err != nil {
				return Check{Name: "tray_indicator", Status: "fail", Detail: "i3/i3bar needs snixembed: sudo pacman -S snixembed, then snixembed --fork before anoted"}
			}
			return Check{Name: "tray_indicator", Status: "warn", Detail: bridge + " — anoted will try to start snixembed automatically"}
		}
		return Check{Name: "tray_indicator", Status: "ok", Detail: "enabled (" + bridge + ")"}
	}
	return Check{Name: "tray_indicator", Status: "ok", Detail: "enabled"}
}
