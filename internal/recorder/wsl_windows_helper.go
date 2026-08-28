//go:build linux

package recorder

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"

	"anoted/internal/config"
)

// WSLHelperRecorder delegates capture to a native Windows helper executable.
type WSLHelperRecorder struct {
	cfg    config.Config
	inner  *DummyRecorder
	helper string
}

func NewWSLHelperRecorder(cfg config.Config) *WSLHelperRecorder {
	helper := cfgExpandHelperPath()
	return &WSLHelperRecorder{
		cfg:    cfg,
		inner:  NewDummyRecorder(),
		helper: helper,
	}
}

func (r *WSLHelperRecorder) Name() string { return "wsl-windows-helper" }

// Unusable always reports a reason: the helper protocol is not implemented.
//
// The previous behaviour was backwards. With the helper installed, Start
// returned an error and nothing recorded; without it, Start fell through to the
// dummy backend and produced a 44-byte WAV while the UI showed a healthy
// recording. Installing the documented dependency made things worse, and the
// silent path was the one that lost meetings. WSL2 cannot capture Windows audio
// today, so say so once, up front.
func (r *WSLHelperRecorder) Unusable() string {
	if r.helper == "" {
		return "WSL2 cannot capture Windows audio; windows-recorder.exe is not installed " +
			"and its protocol is not implemented yet — run anoted natively on Windows"
	}
	return fmt.Sprintf("windows helper found at %s but its protocol is not implemented yet "+
		"— run anoted natively on Windows", r.helper)
}

func (r *WSLHelperRecorder) Start(ctx context.Context, _ SessionConfig) error {
	_ = ctx
	return fmt.Errorf("cannot record: %s", r.Unusable())
}

func (r *WSLHelperRecorder) Stop(ctx context.Context) error {
	return r.inner.Stop(ctx)
}

func (r *WSLHelperRecorder) Status() RecorderStatus {
	st := r.inner.Status()
	if st.Backend == "dummy" {
		st.Backend = r.Name()
	}
	return st
}

func (r *WSLHelperRecorder) helperAvailable(ctx context.Context) bool {
	if r.helper == "" {
		return false
	}
	cmd := exec.CommandContext(ctx, r.helper, "status")
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	var resp map[string]any
	return json.Unmarshal(out, &resp) == nil
}

func cfgExpandHelperPath() string {
	// Typical location when built on Windows and invoked from WSL.
	candidates := []string{
		"/mnt/c/Program Files/anoted/windows-recorder.exe",
		"windows-recorder.exe",
	}
	for _, c := range candidates {
		if _, err := exec.LookPath(c); err == nil {
			return c
		}
	}
	return ""
}
