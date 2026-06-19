//go:build !linux

package transcribe

// GPUInfo describes local GPU capabilities for transcription.
type GPUInfo struct {
	NVIDIA      bool
	Name        string
	Driver      string
	CUDAVersion string
}

// DetectGPU probes for GPU hardware (not implemented on this platform).
func DetectGPU() GPUInfo {
	return GPUInfo{}
}

// NVIDIAAvailable reports whether an NVIDIA GPU is present.
func NVIDIAAvailable() bool {
	return false
}
