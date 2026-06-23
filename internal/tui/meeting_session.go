package tui

import "anoted/internal/detector"

func meetingSessionKey(s detector.MeetingState) string {
	if !s.InMeeting {
		return ""
	}
	return s.Provider + "\x00" + s.Title
}

// detectNewMeetingSession reports whether the poll indicates a new meeting
// (re-join after idle, or a different call while still marked in-meeting).
func detectNewMeetingSession(wasInMeeting bool, lastKey string, state detector.MeetingState) bool {
	if !state.InMeeting {
		return false
	}
	key := meetingSessionKey(state)
	if !wasInMeeting {
		return true
	}
	if lastKey != "" && key != "" && key != lastKey {
		return true
	}
	return false
}
