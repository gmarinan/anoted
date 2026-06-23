//go:build linux

package detector

import (
	"strings"
	"testing"
)

func TestListMicCapturesParse(t *testing.T) {
	sample := `Source Output #1464
	Driver: PipeWire
	Client: 75
	Source: 59
		application.name = "Firefox"
		application.process.binary = "firefox"
		media.name = "Meet - Daily Innovación"
`
	captures, err := parseMicCapturesOutput(sample)
	if err != nil {
		t.Fatal(err)
	}
	if len(captures) != 1 {
		t.Fatalf("expected 1 capture, got %d", len(captures))
	}
	c := captures[0]
	if c.Binary != "firefox" || c.MediaName != "Meet - Daily Innovación" {
		t.Fatalf("unexpected capture: %+v", c)
	}
}

func parseMicCapturesOutput(out string) ([]micCapture, error) {
	var captures []micCapture
	var cur *micCapture
	flush := func() {
		if cur == nil {
			return
		}
		if cur.Binary != "" || cur.AppName != "" {
			captures = append(captures, *cur)
		}
		cur = nil
	}
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "Source Output #") {
			flush()
			cur = &micCapture{}
			continue
		}
		if cur == nil {
			continue
		}
		switch {
		case strings.HasPrefix(line, "application.process.binary = "):
			cur.Binary = unquoteProp(line)
		case strings.HasPrefix(line, "application.name = "):
			cur.AppName = unquoteProp(line)
		case strings.HasPrefix(line, "media.name = "):
			cur.MediaName = unquoteProp(line)
		}
	}
	flush()
	return captures, nil
}
