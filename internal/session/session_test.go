package session

import (
	"testing"
	"time"
)

func TestFormatLocalTimeZero(t *testing.T) {
	if got := FormatLocalTime(time.Time{}, "15:04"); got != "" {
		t.Fatalf("got %q", got)
	}
}

func TestFormatLocalTimeTZ(t *testing.T) {
	t.Setenv("TZ", "America/Santiago")
	utc := time.Date(2026, 6, 19, 4, 6, 0, 0, time.UTC)
	got := FormatLocalTime(utc, "2006-01-02 15:04")
	want := "2026-06-19 00:06"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}
