//go:build windows

package transcribe

import (
	"os/exec"
	"syscall"
	"testing"
)

func TestFormatCmdError_Exit9009(t *testing.T) {
	state := &exec.ExitError{ProcessState: &fakeProcState{code: 9009}}
	err := formatCmdError("create venv", state)
	if err == nil {
		t.Fatal("expected error")
	}
	msg := err.Error()
	if msg == "" {
		t.Fatal("empty message")
	}
}

type fakeProcState struct {
	code int
}

func (f *fakeProcState) String() string    { return "exit status 9009" }
func (f *fakeProcState) ExitCode() int     { return f.code }
func (f *fakeProcState) Sys() any          { return syscall.Errno(9009) }
func (f *fakeProcState) SysUsage() any     { return nil }
func (f *fakeProcState) Success() bool     { return false }
func (f *fakeProcState) SystemTime() int64 { return 0 }
func (f *fakeProcState) UserTime() int64   { return 0 }
