package transcribe

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func queryNVIDIA(nvidiaSMI string, args ...string) string {
	out, err := exec.Command(nvidiaSMI, args...).Output()
	if err != nil {
		return ""
	}
	line := strings.TrimSpace(string(out))
	if idx := strings.IndexByte(line, '\n'); idx >= 0 {
		line = line[:idx]
	}
	return line
}

func findExecutable(names ...string) string {
	for _, name := range names {
		if filepath.IsAbs(name) {
			if info, err := os.Stat(name); err == nil && !info.IsDir() {
				return name
			}
			continue
		}
		if p, err := exec.LookPath(name); err == nil {
			return p
		}
	}
	return ""
}

func gpuInfoFromSMI(path string) GPUInfo {
	name := queryNVIDIA(path, "--query-gpu=name", "--format=csv,noheader")
	driver := queryNVIDIA(path, "--query-gpu=driver_version", "--format=csv,noheader")
	cuda := strings.TrimSpace(queryNVIDIA(path, "--query-gpu=cuda_version", "--format=csv,noheader"))
	if name == "" {
		name = "NVIDIA GPU"
	}
	return GPUInfo{
		NVIDIA:      true,
		Name:        name,
		Driver:      driver,
		CUDAVersion: cuda,
	}
}
