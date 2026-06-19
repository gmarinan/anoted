//go:build linux

package recorder

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"

	"meetctl/internal/config"
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

func (r *WSLHelperRecorder) Start(ctx context.Context, sess SessionConfig) error {
	if r.helperAvailable(ctx) {
		// Future: JSON-RPC over stdin/stdout to windows-recorder.exe
		return fmt.Errorf("windows helper found at %s but protocol not yet implemented; using dummy backend", r.helper)
	}
	return r.inner.Start(ctx, sess)
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
		"/mnt/c/Program Files/meetctl/windows-recorder.exe",
		"windows-recorder.exe",
	}
	for _, c := range candidates {
		if _, err := exec.LookPath(c); err == nil {
			return c
		}
	}
	return ""
}
