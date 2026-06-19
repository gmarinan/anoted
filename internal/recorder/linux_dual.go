//go:build linux

package recorder

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
)

// SessionAudioFile is the mixed system + microphone recording filename.
const SessionAudioFile = "recording.wav"

// startDualCapture records system monitor and microphone in a single process.
// Two parallel pw-cat/ffmpeg instances conflict on PipeWire and may capture the same source.
func startDualCapture(devs resolvedDevices, sess SessionConfig, dir string) (*exec.Cmd, error) {
	if _, err := exec.LookPath("ffmpeg"); err == nil {
		return startDualFFmpeg(devs, sess, dir)
	}
	return startDualParec(devs, sess, dir)
}

func startDualFFmpeg(devs resolvedDevices, sess SessionConfig, dir string) (*exec.Cmd, error) {
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
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start ffmpeg dual capture: %w", err)
	}
	return cmd, nil
}

func startDualParec(devs resolvedDevices, sess SessionConfig, dir string) (*exec.Cmd, error) {
	rate := outputSampleRate(sess)
	channels := outputChannels(sess)
	systemRaw := dirFile(dir, "system.raw")
	micRaw := dirFile(dir, "mic.raw")
	outWav := dirFile(dir, SessionAudioFile)
	script := fmt.Sprintf(`set -e
parec --record -d %q --format=s16le --rate=%d --channels=%d > %q &
P1=$!
parec --record -d %q --format=s16le --rate=%d --channels=1 > %q &
P2=$!
trap 'kill -INT $P1 $P2 2>/dev/null; wait $P1 $P2; ffmpeg -nostats -loglevel error -y -f s16le -ar %d -ac %d -i %q -f s16le -ar %d -ac 1 -i %q -filter_complex "[0:a][1:a]amix=inputs=2:duration=longest:dropout_transition=2" -ar %d -ac %d %q; rm -f %q %q' INT TERM EXIT
wait $P1 $P2
`,
		devs.system, rate, channels, systemRaw,
		devs.mic, rate, micRaw,
		rate, channels, systemRaw, rate, micRaw, rate, channels, outWav,
		systemRaw, micRaw,
	)
	cmd := exec.Command("bash", "-c", script)
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start parec dual capture: %w", err)
	}
	return cmd, nil
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

func stopCaptureCmd(cmd *exec.Cmd) {
	if cmd == nil || cmd.Process == nil {
		return
	}
	_ = cmd.Process.Signal(os.Interrupt)
	_ = cmd.Wait()
}
