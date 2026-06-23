package logging

import (
	"log/slog"
	"testing"
)

func TestSetupFileReturnsLogger(t *testing.T) {
	logger, err := SetupFile(slog.LevelInfo)
	if err != nil {
		t.Fatal(err)
	}
	if logger == nil {
		t.Fatal("expected logger")
	}
}

func TestSetupReturnsLogger(t *testing.T) {
	logger, err := Setup(slog.LevelInfo)
	if err != nil {
		t.Fatal(err)
	}
	if logger == nil {
		t.Fatal("expected logger")
	}
}
