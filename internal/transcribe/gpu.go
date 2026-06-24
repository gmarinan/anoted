package transcribe

// GPUInfo describes local GPU capabilities for transcription.
type GPUInfo struct {
	NVIDIA      bool
	Name        string
	Driver      string
	CUDAVersion string
}
