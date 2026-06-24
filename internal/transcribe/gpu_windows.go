//go:build windows

package transcribe

import (
	"os"
	"path/filepath"
)

// DetectGPU probes for an NVIDIA GPU via nvidia-smi on Windows.
func DetectGPU() GPUInfo {
	path := findNvidiaSMIWindows()
	if path == "" {
		return GPUInfo{}
	}
	return gpuInfoFromSMI(path)
}

// NVIDIAAvailable reports whether an NVIDIA GPU is present.
func NVIDIAAvailable() bool {
	return DetectGPU().NVIDIA
}

func findNvidiaSMIWindows() string {
	candidates := []string{"nvidia-smi"}
	if pf := os.Getenv("ProgramFiles"); pf != "" {
		candidates = append(candidates, filepath.Join(pf, "NVIDIA Corporation", "NVSMI", "nvidia-smi.exe"))
	}
	if pf86 := os.Getenv("ProgramFiles(x86)"); pf86 != "" {
		candidates = append(candidates, filepath.Join(pf86, "NVIDIA Corporation", "NVSMI", "nvidia-smi.exe"))
	}
	if windir := os.Getenv("WINDIR"); windir != "" {
		candidates = append(candidates, filepath.Join(windir, "System32", "nvidia-smi.exe"))
	}
	return findExecutable(candidates...)
}
