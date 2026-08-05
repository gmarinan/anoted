//go:build linux

package recorder

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	// captureStartGrace is how long we wait after spawning ffmpeg before
	// declaring the recording healthy. ffmpeg opens and validates its inputs
	// during startup, so a bad device or unsupported format shows up here
	// rather than as a session that silently records nothing.
	captureStartGrace = 400 * time.Millisecond
	// captureStopTimeout bounds the wait for ffmpeg to flush and close the
	// output file after SIGINT before we kill it.
	captureStopTimeout = 5 * time.Second
	// captureStderrLimit caps retained child diagnostics; ffmpeg can be chatty
	// and a recording runs for hours.
	captureStderrLimit = 8 << 10
)

// startDualCapture records the system monitor and the microphone into a single
// ffmpeg process. Two parallel pw-cat/ffmpeg instances conflict on PipeWire and
// may end up capturing the same source.
func startDualCapture(devs resolvedDevices, sess SessionConfig, dir string) (*captureProc, error) {
	return startDualFFmpeg(devs, sess, dir)
}

func startDualFFmpeg(devs resolvedDevices, sess SessionConfig, dir string) (*captureProc, error) {
	rate := strconv.Itoa(outputSampleRate(sess))
	channels := strconv.Itoa(outputChannels(sess))
	args := []string{
		"-nostats", "-loglevel", "error",
		"-y",
		"-f", "pulse", "-i", devs.system,
		"-f", "pulse", "-i", devs.mic,
		"-filter_complex", "[0:a][1:a]amix=inputs=2:duration=longest:dropout_transition=2",
		"-ar", rate,
		"-ac", channels,
		dirFile(dir, SessionAudioFile),
	}
	cmd := exec.Command("ffmpeg", args...)
	stderr := &boundedBuffer{limit: captureStderrLimit}
	cmd.Stderr = stderr

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start ffmpeg dual capture: %w", err)
	}

	c := &captureProc{cmd: cmd, stderr: stderr, done: make(chan struct{})}
	go c.reap()

	select {
	case <-c.done:
		return nil, fmt.Errorf("ffmpeg dual capture exited on startup: %w", c.exitError())
	case <-time.After(captureStartGrace):
	}
	return c, nil
}

// captureProc owns a running capture child process.
//
// exec.Cmd.Start only reports a failure to fork, so without this the recorder
// could not tell a healthy capture from one that died seconds in — leaving the
// TUI showing a growing duration over a file nothing is writing to.
type captureProc struct {
	cmd    *exec.Cmd
	stderr *boundedBuffer
	done   chan struct{}

	mu       sync.Mutex
	waitErr  error
	stopping bool
}

func (c *captureProc) reap() {
	err := c.cmd.Wait()
	c.mu.Lock()
	c.waitErr = err
	c.mu.Unlock()
	close(c.done)
}

// Err reports why capture ended unexpectedly. It returns nil while capture is
// still running and after a deliberate Stop, so callers can poll it to decide
// whether the session is still healthy.
func (c *captureProc) Err() error {
	if c == nil {
		return nil
	}
	select {
	case <-c.done:
	default:
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.stopping || c.waitErr == nil {
		return nil
	}
	return c.annotate(c.waitErr)
}

func (c *captureProc) exitError() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.waitErr == nil {
		return fmt.Errorf("exited without error")
	}
	return c.annotate(c.waitErr)
}

// annotate must be called with c.mu held.
func (c *captureProc) annotate(err error) error {
	if tail := c.stderr.String(); tail != "" {
		return fmt.Errorf("%w: %s", err, tail)
	}
	return err
}

// Stop interrupts capture and waits for ffmpeg to flush and close the output
// file. It reports a capture that had already died on its own, since that means
// the recording is incomplete.
func (c *captureProc) Stop() error {
	if c == nil || c.cmd.Process == nil {
		return nil
	}
	prior := c.Err()

	c.mu.Lock()
	c.stopping = true
	c.mu.Unlock()

	_ = c.cmd.Process.Signal(os.Interrupt)
	select {
	case <-c.done:
	case <-time.After(captureStopTimeout):
		// ffmpeg ignored SIGINT; killing it leaves a WAV with a stale header,
		// which is still better than never returning from Stop.
		_ = c.cmd.Process.Kill()
		<-c.done
		return fmt.Errorf("capture did not exit within %s; recording may be truncated", captureStopTimeout)
	}
	return prior
}

func outputSampleRate(sess SessionConfig) int {
	if sess.SampleRate > 0 {
		return sess.SampleRate
	}
	return 48000
}

func outputChannels(sess SessionConfig) int {
	if sess.Channels > 0 {
		return sess.Channels
	}
	return 2
}

// boundedBuffer retains at most the last limit bytes written to it.
type boundedBuffer struct {
	mu    sync.Mutex
	limit int
	buf   []byte
}

func (b *boundedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.buf = append(b.buf, p...)
	if len(b.buf) > b.limit {
		b.buf = b.buf[len(b.buf)-b.limit:]
	}
	return len(p), nil
}

func (b *boundedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return strings.TrimSpace(string(b.buf))
}
