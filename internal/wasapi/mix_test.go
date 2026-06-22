//go:build windows

package wasapi

import "testing"

func TestMixS16(t *testing.T) {
	a := []byte{0, 0, 10, 0}
	b := []byte{0, 0, 20, 0}
	out := MixS16(nil, a, b)
	if len(out) != 4 {
		t.Fatalf("len %d", len(out))
	}
	if out[2] != 15 || out[3] != 0 {
		t.Fatalf("mixed sample: %v", out)
	}
}
