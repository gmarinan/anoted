//go:build linux

package recorder

import (
	"os/exec"
	"strings"
	"testing"
)

func newTestCapture(t *testing.T, shell string) *captureProc {
	t.Helper()
	cmd := exec.Command("sh", "-c", shell)
	stderr := &boundedBuffer{limit: captureStderrLimit}
	cmd.Stderr = stderr
	if err := cmd.Start(); err != nil {
		t.Fatalf("start helper: %v", err)
	}
	c := &captureProc{cmd: cmd, stderr: stderr, done: make(chan struct{})}
	go c.reap()
	return c
}

func TestCaptureProcReportsUnexpectedExit(t *testing.T) {
	// A capture that dies mid-meeting used to leave the UI showing a healthy,
	// growing recording, because Start() only reports fork failures.
	c := newTestCapture(t, "echo 'device busy' >&2; exit 3")
	<-c.done

	err := c.Err()
	if err == nil {
		t.Fatal("Err() = nil for a capture that exited on its own, want an error")
	}
	if !strings.Contains(err.Error(), "device busy") {
		t.Fatalf("Err() = %q, want it to carry the child's stderr", err)
	}
}

func TestCaptureProcStopIsNotAnError(t *testing.T) {
	// A deliberate Stop kills the child, so Wait reports a signal; that must
	// not be surfaced as a failed recording.
	c := newTestCapture(t, "sleep 30")
	if err := c.Stop(); err != nil {
		t.Fatalf("Stop() = %v, want nil for a deliberate stop", err)
	}
	if err := c.Err(); err != nil {
		t.Fatalf("Err() = %v after deliberate stop, want nil", err)
	}
}

func TestCaptureProcErrNilWhileRunning(t *testing.T) {
	c := newTestCapture(t, "sleep 30")
	defer c.Stop()
	if err := c.Err(); err != nil {
		t.Fatalf("Err() = %v while still running, want nil", err)
	}
}

func TestBoundedBufferKeepsTail(t *testing.T) {
	b := &boundedBuffer{limit: 8}
	b.Write([]byte("aaaaaaaaaaaaaaaa"))
	b.Write([]byte("tail"))
	got := b.String()
	if len(got) > 8 {
		t.Fatalf("buffer grew past limit: %q", got)
	}
	if !strings.HasSuffix(got, "tail") {
		t.Fatalf("String() = %q, want the most recent bytes", got)
	}
}
