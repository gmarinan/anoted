//go:build !linux

package tray

// EnsureLinuxBridge is a no-op off Linux.
func EnsureLinuxBridge() error {
	return nil
}

// LinuxBridgeDetail returns empty off Linux.
func LinuxBridgeDetail() string {
	return ""
}
