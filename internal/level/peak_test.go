package level

import (
	"math"
	"testing"
)

func TestPeakS16LE(t *testing.T) {
	tests := []struct {
		name string
		buf  []byte
		want float64
	}{
		{"empty", nil, 0},
		{"silence", []byte{0, 0, 0, 0}, 0},
		{"max positive", []byte{0xFF, 0x7F}, 1},
		{"max negative", []byte{0x00, 0x80}, 1},
		{"half", []byte{0x00, 0x40}, 0.5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := peakS16LE(tt.buf)
			if math.Abs(got-tt.want) > 0.02 {
				t.Fatalf("peakS16LE() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSmoothPeak(t *testing.T) {
	rise := smoothPeak(0, 0.8)
	if rise != 0.8 {
		t.Fatalf("attack: got %v want 0.8", rise)
	}
	fall := smoothPeak(0.8, 0.1)
	if fall >= 0.8 {
		t.Fatalf("release should decay: got %v", fall)
	}
	if fall < 0.1 {
		t.Fatalf("release should not go below sample: got %v", fall)
	}
}
