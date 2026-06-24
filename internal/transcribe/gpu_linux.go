//go:build linux

package transcribe

// DetectGPU probes for an NVIDIA GPU via nvidia-smi (checks PATH and /usr/bin).
func DetectGPU() GPUInfo {
	path := findExecutable("nvidia-smi", "/usr/bin/nvidia-smi")
	if path == "" {
		return GPUInfo{}
	}
	return gpuInfoFromSMI(path)
}

// NVIDIAAvailable reports whether an NVIDIA GPU is present.
func NVIDIAAvailable() bool {
	return DetectGPU().NVIDIA
}
