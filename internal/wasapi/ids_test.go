//go:build windows

package wasapi

import (
	"testing"

	"github.com/gen2brain/malgo"
)

func TestParseLoopbackID(t *testing.T) {
	var id malgo.DeviceID
	id[0] = 0xab
	id[1] = 0xcd
	stored := LoopbackID(id)
	got, err := ParseLoopbackID(stored)
	if err != nil {
		t.Fatal(err)
	}
	if got != id {
		t.Fatalf("got %#v want %#v", got, id)
	}
}

func TestParseCaptureIDInvalid(t *testing.T) {
	if _, err := ParseCaptureID("bad"); err == nil {
		t.Fatal("expected error")
	}
}
