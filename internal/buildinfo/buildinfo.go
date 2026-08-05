// Package buildinfo exposes the version stamped into the binary at build time.
package buildinfo

import (
	"runtime/debug"
	"sync"
)

// version is stamped by the linker:
//
//	-ldflags "-X anoted/internal/buildinfo.version=v1.2.3"
//
// The Makefile derives it from `git describe`.
var version string

// resolved falls back to the VCS data the go tool embeds automatically, so a
// plain `go build` or `go install` still identifies which commit is running
// instead of reporting a useless bare "dev".
var resolved = sync.OnceValue(func() string {
	if version != "" {
		return version
	}
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "dev"
	}
	var rev, dirty string
	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.revision":
			if len(s.Value) >= 7 {
				rev = s.Value[:7]
			}
		case "vcs.modified":
			if s.Value == "true" {
				dirty = "-dirty"
			}
		}
	}
	if rev == "" {
		return "dev"
	}
	return rev + dirty
})

// Version reports the build version, e.g. "v0.3.1", "50323c9-dirty" or "dev".
func Version() string { return resolved() }
