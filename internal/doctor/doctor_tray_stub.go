//go:build !linux

package doctor

func hasStatusNotifierWatcherForDoctor() bool {
	return true
}
