package folderpicker

import (
	"os"
	"testing"
)

func TestResolveStartDirEmpty(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	got := resolveStartDir("")
	if got != home {
		t.Fatalf("got %q want %q", got, home)
	}
}

func TestResolveStartDirValid(t *testing.T) {
	dir := t.TempDir()
	got := resolveStartDir(dir)
	if got != dir {
		t.Fatalf("got %q want %q", got, dir)
	}
}
