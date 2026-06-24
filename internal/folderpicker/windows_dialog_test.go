//go:build linux || windows

package folderpicker

import (
	"strings"
	"testing"
)

func TestWindowsFolderScriptEscapesQuotes(t *testing.T) {
	script := windowsFolderScript("C:\\Users\\O'Brien\\Music")
	if !strings.Contains(script, "O''Brien") {
		t.Fatalf("expected escaped quotes in script:\n%s", script)
	}
}
