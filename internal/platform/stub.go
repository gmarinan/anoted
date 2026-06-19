//go:build !linux && !windows

package platform

func detect() Info {
	return Info{
		OS:          OSUnknown,
		DisplayName: "Unknown",
		Session:     "unknown",
	}
}
