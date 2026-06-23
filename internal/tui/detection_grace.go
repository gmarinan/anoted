package tui

import (
	"time"

	"anoted/internal/config"
)

const autoRestartCooldown = 5 * time.Second

func meetingEndGrace(cfg config.Config) time.Duration {
	ms := cfg.Detection.MeetingEndGraceMS
	if ms <= 0 {
		ms = 6000
	}
	return time.Duration(ms) * time.Millisecond
}

func shouldStopForMeetingEnd(inMeeting bool, absentSince, now time.Time, grace time.Duration) (stop bool, updatedAbsentSince time.Time) {
	if inMeeting {
		return false, time.Time{}
	}
	if absentSince.IsZero() {
		return false, now
	}
	if now.Sub(absentSince) >= grace {
		return true, time.Time{}
	}
	return false, absentSince
}

func shouldBlockAutoStart(recordOpInFlight bool, lastAutoStop, now time.Time) bool {
	if recordOpInFlight {
		return true
	}
	if !lastAutoStop.IsZero() && now.Sub(lastAutoStop) < autoRestartCooldown {
		return true
	}
	return false
}
