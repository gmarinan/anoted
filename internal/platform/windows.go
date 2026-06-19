//go:build windows

package platform

func detect() Info {
	return Info{
		OS:          OSWindows,
		DisplayName: "Windows",
		Session:     "windows",
	}
}
