package doctor

import (
	"strings"
	"testing"

	"anoted/internal/config"
)

func TestRun(t *testing.T) {
	rep := Run(config.Default())
	if rep.Platform.Name() == "" {
		t.Fatal("expected platform name")
	}
	if len(rep.Checks) == 0 {
		t.Fatal("expected checks")
	}
	text := Format(rep)
	if !strings.Contains(text, "anoted doctor") {
		t.Fatal("expected formatted header")
	}
}
