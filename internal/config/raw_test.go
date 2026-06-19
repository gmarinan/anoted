package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSaveRawValidatesYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	_, err := SaveRaw(path, "auto_record: [not a bool]\n")
	if err == nil {
		t.Fatal("expected yaml validation error")
	}
}

func TestReadRawMissingFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "missing.yaml")
	raw, err := ReadRaw(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(raw, "auto_record:") {
		t.Fatalf("expected default yaml, got %q", raw)
	}
}

func TestSaveRawRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	content := "auto_record: true\noutput_dir: ~/Music/meetctl\n"
	cfg, err := SaveRaw(path, content)
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.AutoRecord {
		t.Fatal("expected auto_record true")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != content {
		t.Fatalf("expected raw content preserved, got %q", string(data))
	}
}
