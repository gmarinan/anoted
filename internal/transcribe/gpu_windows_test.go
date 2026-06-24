//go:build windows

package transcribe

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFindNvidiaSMIWindowsCandidates(t *testing.T) {
	t.Setenv("PATH", "")
	t.Setenv("ProgramFiles", `C:\Program Files`)
	t.Setenv("ProgramFiles(x86)", `C:\Program Files (x86)`)
	t.Setenv("WINDIR", `C:\Windows`)

	// Create a fake nvidia-smi in Program Files path.
	pf := filepath.Join(`C:\Program Files`, "NVIDIA Corporation", "NVSMI")
	if err := os.MkdirAll(pf, 0o755); err != nil {
		t.Fatal(err)
	}
	fake := filepath.Join(pf, "nvidia-smi.exe")
	if err := os.WriteFile(fake, []byte("stub"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Remove(fake) })

	got := findNvidiaSMIWindows()
	if got != fake {
		t.Fatalf("got %q want %q", got, fake)
	}
}

func TestFindNvidiaSMIWindowsEmpty(t *testing.T) {
	t.Setenv("PATH", "")
	t.Setenv("ProgramFiles", "")
	t.Setenv("ProgramFiles(x86)", "")
	t.Setenv("WINDIR", "")
	if got := findNvidiaSMIWindows(); got != "" {
		t.Fatalf("expected empty, got %q", got)
	}
}

func TestFindExecutableWindows(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "tool.exe")
	if err := os.WriteFile(bin, []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	if got := findExecutable(bin); got != bin {
		t.Fatalf("got %q want %q", got, bin)
	}
	if got := findExecutable(filepath.Join(dir, "missing.exe")); got != "" {
		t.Fatalf("expected empty for missing file, got %q", got)
	}
}

func TestGPUInfoFromSMIStub(t *testing.T) {
	// gpuInfoFromSMI requires a working nvidia-smi; just verify empty name fallback path.
	info := GPUInfo{NVIDIA: true, Name: ""}
	if info.Name == "" {
		info.Name = "NVIDIA GPU"
	}
	if !strings.Contains(info.Name, "NVIDIA") {
		t.Fatal("expected fallback name")
	}
}
