package smtp

import (
	"fmt"
	"time"

	"tackle/internal/repositories"
)

// IsWithinSendWindow returns true if now is within the configured send window.
// It handles overnight windows (start > end means the window crosses midnight).
// If no window is configured (nil start/end or empty timezone), always returns true.
func IsWithinSendWindow(schedule repositories.CampaignSendSchedule, now time.Time) bool {
	if schedule.SendWindowStart == nil || schedule.SendWindowEnd == nil {
		return true // no window configured
	}

	loc := resolveTimezone(schedule.SendWindowTimezone)
	local := now.In(loc)

	// Check day of week restriction.
	if len(schedule.SendWindowDays) > 0 {
		dayOK := false
		weekday := int(local.Weekday()) // 0=Sun, 6=Sat
		for _, d := range schedule.SendWindowDays {
			if d == weekday {
				dayOK = true
				break
			}
		}
		if !dayOK {
			return false
		}
	}

	start, err := parseTimeOfDay(*schedule.SendWindowStart)
	if err != nil {
		return true // misconfigured, fail open
	}
	end, err := parseTimeOfDay(*schedule.SendWindowEnd)
	if err != nil {
		return true
	}

	currentMins := local.Hour()*60 + local.Minute()
	startMins := start.Hour*60 + start.Minute
	endMins := end.Hour*60 + end.Minute

	if startMins <= endMins {
		// Normal window: e.g. 09:00 – 17:00
		return currentMins >= startMins && currentMins < endMins
	}
	// Overnight window: e.g. 22:00 – 06:00
	return currentMins >= startMins || currentMins < endMins
}

// NextWindowOpen returns the next time the send window opens after now.
// Returns now immediately if already within the window.
func NextWindowOpen(schedule repositories.CampaignSendSchedule, now time.Time) time.Time {
	if IsWithinSendWindow(schedule, now) {
		return now
	}
	if schedule.SendWindowStart == nil || schedule.SendWindowEnd == nil {
		return now
	}

	loc := resolveTimezone(schedule.SendWindowTimezone)
	local := now.In(loc)
	start, err := parseTimeOfDay(*schedule.SendWindowStart)
	if err != nil {
		return now
	}

	// Try today's window open.
	candidate := time.Date(local.Year(), local.Month(), local.Day(),
		start.Hour, start.Minute, 0, 0, loc)
	if candidate.After(now) && isValidDay(schedule, candidate) {
		return candidate
	}

	// Advance day-by-day until we find a valid window day.
	for i := 1; i <= 8; i++ {
		candidate = candidate.AddDate(0, 0, 1)
		if isValidDay(schedule, candidate) {
			return candidate
		}
	}
	return candidate
}

// TimeUntilWindowClose returns the duration remaining until the send window closes.
// Returns 0 if not within a window.
func TimeUntilWindowClose(schedule repositories.CampaignSendSchedule, now time.Time) time.Duration {
	if !IsWithinSendWindow(schedule, now) {
		return 0
	}
	if schedule.SendWindowEnd == nil {
		return time.Duration(1<<63 - 1) // effectively infinite
	}

	loc := resolveTimezone(schedule.SendWindowTimezone)
	local := now.In(loc)
	end, err := parseTimeOfDay(*schedule.SendWindowEnd)
	if err != nil {
		return 0
	}

	closeToday := time.Date(local.Year(), local.Month(), local.Day(),
		end.Hour, end.Minute, 0, 0, loc)

	// Handle overnight: if end < start, closing time is tomorrow.
	if schedule.SendWindowStart != nil {
		start, err := parseTimeOfDay(*schedule.SendWindowStart)
		if err == nil {
			startMins := start.Hour*60 + start.Minute
			endMins := end.Hour*60 + end.Minute
			if endMins < startMins {
				closeToday = closeToday.AddDate(0, 0, 1)
			}
		}
	}

	d := closeToday.Sub(now)
	if d < 0 {
		return 0
	}
	return d
}

// --- helpers ---

type timeOfDay struct {
	Hour   int
	Minute int
}

// parseTimeOfDay parses a "HH:MM" or "HH:MM:SS" string.
func parseTimeOfDay(s string) (timeOfDay, error) {
	var h, m, sec int
	n, err := fmt.Sscanf(s, "%d:%d:%d", &h, &m, &sec)
	if err != nil || n < 2 {
		n2, err2 := fmt.Sscanf(s, "%d:%d", &h, &m)
		if err2 != nil || n2 < 2 {
			return timeOfDay{}, fmt.Errorf("invalid time-of-day %q", s)
		}
	}
	return timeOfDay{Hour: h, Minute: m}, nil
}

// resolveTimezone loads the IANA timezone location. Falls back to UTC on error.
func resolveTimezone(tz *string) *time.Location {
	if tz == nil || *tz == "" {
		return time.UTC
	}
	loc, err := time.LoadLocation(*tz)
	if err != nil {
		return time.UTC
	}
	return loc
}

// isValidDay returns true if the candidate time falls on a valid send-window day.
func isValidDay(schedule repositories.CampaignSendSchedule, t time.Time) bool {
	if len(schedule.SendWindowDays) == 0 {
		return true
	}
	weekday := int(t.Weekday())
	for _, d := range schedule.SendWindowDays {
		if d == weekday {
			return true
		}
	}
	return false
}
