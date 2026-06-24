//go:build !linux && !windows

package transcribe

// DetectGPU probes for GPU hardware (not implemented on this platform).
func DetectGPU() GPUInfo {
	return GPUInfo{}
}

// NVIDIAAvailable reports whether an NVIDIA GPU is present.
func NVIDIAAvailable() bool {
	return false
}
