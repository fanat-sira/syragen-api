package keys

import (
	"testing"
	"time"
)

func TestPeriodCalculation(t *testing.T) {
	loc, err := time.LoadLocation("Europe/Moscow")
	if err != nil {
		t.Fatalf("failed to load Europe/Moscow location: %v", err)
	}

	tests := []struct {
		name          string
		inputTime     time.Time
		expectedID    string
		expectedStart time.Time
		expectedEnd   time.Time
	}{
		{
			name:          "Before 09:00 MSK (08:59:59)",
			inputTime:     time.Date(2026, 8, 26, 8, 59, 59, 0, loc),
			expectedID:    "2026-08-25",
			expectedStart: time.Date(2026, 8, 25, 9, 0, 0, 0, loc),
			expectedEnd:   time.Date(2026, 8, 26, 9, 0, 0, 0, loc),
		},
		{
			name:          "Exactly 09:00:00 MSK",
			inputTime:     time.Date(2026, 8, 26, 9, 0, 0, 0, loc),
			expectedID:    "2026-08-26",
			expectedStart: time.Date(2026, 8, 26, 9, 0, 0, 0, loc),
			expectedEnd:   time.Date(2026, 8, 27, 9, 0, 0, 0, loc),
		},
		{
			name:          "After 09:00 MSK (16:00:00)",
			inputTime:     time.Date(2026, 8, 25, 16, 0, 0, 0, loc),
			expectedID:    "2026-08-25",
			expectedStart: time.Date(2026, 8, 25, 9, 0, 0, 0, loc),
			expectedEnd:   time.Date(2026, 8, 26, 9, 0, 0, 0, loc),
		},
		{
			name:          "Midnight 00:00 MSK",
			inputTime:     time.Date(2026, 8, 26, 0, 0, 0, 0, loc),
			expectedID:    "2026-08-25",
			expectedStart: time.Date(2026, 8, 25, 9, 0, 0, 0, loc),
			expectedEnd:   time.Date(2026, 8, 26, 9, 0, 0, 0, loc),
		},
		{
			name:          "Month Rollover (Aug 31 23:59)",
			inputTime:     time.Date(2026, 8, 31, 23, 59, 59, 0, loc),
			expectedID:    "2026-08-31",
			expectedStart: time.Date(2026, 8, 31, 9, 0, 0, 0, loc),
			expectedEnd:   time.Date(2026, 9, 1, 9, 0, 0, 0, loc),
		},
		{
			name:          "Month Rollover Morning (Sep 1 08:00)",
			inputTime:     time.Date(2026, 9, 1, 8, 0, 0, 0, loc),
			expectedID:    "2026-08-31",
			expectedStart: time.Date(2026, 8, 31, 9, 0, 0, 0, loc),
			expectedEnd:   time.Date(2026, 9, 1, 9, 0, 0, 0, loc),
		},
		{
			name:          "Year Rollover (Dec 31 22:00)",
			inputTime:     time.Date(2026, 12, 31, 22, 0, 0, 0, loc),
			expectedID:    "2026-12-31",
			expectedStart: time.Date(2026, 12, 31, 9, 0, 0, 0, loc),
			expectedEnd:   time.Date(2027, 1, 1, 9, 0, 0, 0, loc),
		},
		{
			name:          "Year Rollover Morning (Jan 1 08:30)",
			inputTime:     time.Date(2027, 1, 1, 8, 30, 0, 0, loc),
			expectedID:    "2026-12-31",
			expectedStart: time.Date(2026, 12, 31, 9, 0, 0, 0, loc),
			expectedEnd:   time.Date(2027, 1, 1, 9, 0, 0, 0, loc),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := GetPeriod(tc.inputTime, loc)
			if p.ID != tc.expectedID {
				t.Errorf("expected period ID %s, got %s", tc.expectedID, p.ID)
			}
			if !p.StartTime.Equal(tc.expectedStart) {
				t.Errorf("expected start time %s, got %s", tc.expectedStart, p.StartTime)
			}
			if !p.EndTime.Equal(tc.expectedEnd) {
				t.Errorf("expected end time %s, got %s", tc.expectedEnd, p.EndTime)
			}

			next := NextBoundary(tc.inputTime, loc)
			if !next.Equal(tc.expectedEnd) {
				t.Errorf("expected next boundary %s, got %s", tc.expectedEnd, next)
			}
		})
	}
}
