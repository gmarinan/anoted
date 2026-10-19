package session

import (
	"testing"
	"time"
	_ "time/tzdata" // LoadLocation must not depend on the runner's zoneinfo
)

func TestFormatLocalTimeZero(t *testing.T) {
	if got := FormatLocalTime(time.Time{}, "15:04"); got != "" {
		t.Fatalf("got %q", got)
	}
}

func TestFormatLocalTimeTZ(t *testing.T) {
	// time.Local is resolved once at startup, so setting TZ here has no effect;
	// the test only passed on machines already in Chile. Swap Local directly.
	loc, err := time.LoadLocation("America/Santiago")
	if err != nil {
		t.Fatal(err)
	}
	orig := time.Local
	time.Local = loc
	t.Cleanup(func() { time.Local = orig })
	utc := time.Date(2026, 6, 19, 4, 6, 0, 0, time.UTC)
	got := FormatLocalTime(utc, "2006-01-02 15:04")
	want := "2026-06-19 00:06"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}
