//go:build !linux && !windows

package autostart

func available() bool {
	return false
}

func enabled() bool {
	return false
}

func entryPath() (string, error) {
	return "", ErrUnavailable
}

func enable(entry Entry) error {
	return ErrUnavailable
}

func disable() error {
	return ErrUnavailable
}
