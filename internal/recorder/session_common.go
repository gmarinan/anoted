package recorder

import (
	"fmt"
	"path/filepath"
	"time"
)

// SessionAudioFile is the mixed system + microphone recording filename.
const SessionAudioFile = "recording.wav"

func sessionDir(sess SessionConfig) string {
	name := fmt.Sprintf("%s_%s", time.Now().Format("2006-01-02_15-04-05"), sess.Provider)
	return filepath.Join(sess.OutputRoot, name)
}

func dirFile(dir, name string) string {
	return filepath.Join(dir, name)
}
