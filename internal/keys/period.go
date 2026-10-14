package keys

import (
	"time"
)

// Period represents a daily accounting window from 09:00 MSK to next 09:00 MSK.
type Period struct {
	ID        string    // Moscow calendar date of period start: "YYYY-MM-DD"
	StartTime time.Time // 09:00:00 MSK at start of period
	EndTime   time.Time // 09:00:00 MSK at end of period
}

// GetPeriod returns the accounting period for a given timestamp and timezone location.
func GetPeriod(t time.Time, loc *time.Location) Period {
	if loc == nil {
		loc = time.UTC
	}
	tInLoc := t.In(loc)
	year, month, day := tInLoc.Date()

	boundaryToday := time.Date(year, month, day, 9, 0, 0, 0, loc)

	var startTime, endTime time.Time
	if tInLoc.Before(boundaryToday) {
		// Before 09:00 MSK today: period started yesterday at 09:00 MSK and ends today at 09:00 MSK
		startTime = boundaryToday.AddDate(0, 0, -1)
		endTime = boundaryToday
	} else {
		// At or after 09:00 MSK today: period started today at 09:00 MSK and ends tomorrow at 09:00 MSK
		startTime = boundaryToday
		endTime = boundaryToday.AddDate(0, 0, 1)
	}

	return Period{
		ID:        startTime.Format("2006-01-02"),
		StartTime: startTime,
		EndTime:   endTime,
	}
}

// NextBoundary returns the upcoming 09:00 MSK timestamp strictly after time t.
func NextBoundary(t time.Time, loc *time.Location) time.Time {
	p := GetPeriod(t, loc)
	return p.EndTime
}

// DurationUntilNextBoundary returns the duration from now until the next 09:00 MSK boundary.
func DurationUntilNextBoundary(t time.Time, loc *time.Location) time.Duration {
	next := NextBoundary(t, loc)
	dur := next.Sub(t)
	if dur <= 0 {
		return time.Second
	}
	return dur
}
