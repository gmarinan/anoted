//go:build !windows

package session

import "testing"

func TestIsAnotedCmdline(t *testing.T) {
	cases := []struct {
		name string
		cmd  string
		want bool
	}{
		{"deployed path", "/home/x/.local/bin/anoted\x00watch", true},
		{"bare name", "anoted\x00watch", true},
		{"dev build", "/tmp/anoted-fix\x00watch", true},
		{"another process", "/usr/bin/docker-proxy\x00-proto\x00tcp", false},
		{"kernel thread (empty)", "", false},
	}
	for _, c := range cases {
		if got := isAnotedCmdline([]byte(c.cmd)); got != c.want {
			t.Errorf("%s: isAnotedCmdline(%q) = %v, want %v", c.name, c.cmd, got, c.want)
		}
	}
}
