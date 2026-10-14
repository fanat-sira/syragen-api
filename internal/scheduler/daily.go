package scheduler

import (
	"context"
	"log"
	"syragen-apibased/internal/keys"
	"time"
)

// DailyScheduler manages the 09:00 MSK rollover trigger without drift.
type DailyScheduler struct {
	loc        *time.Location
	onRollover func(ctx context.Context)
}

// NewDailyScheduler creates a new DailyScheduler.
func NewDailyScheduler(loc *time.Location, onRollover func(ctx context.Context)) *DailyScheduler {
	if loc == nil {
		loc = time.UTC
	}
	return &DailyScheduler{
		loc:        loc,
		onRollover: onRollover,
	}
}

// Start runs the scheduler loop in the background until the context is cancelled.
func (s *DailyScheduler) Start(ctx context.Context) {
	go func() {
		for {
			now := time.Now()
			dur := keys.DurationUntilNextBoundary(now, s.loc)
			nextBoundary := keys.NextBoundary(now, s.loc)

			log.Printf("[scheduler] next 09:00 MSK boundary at %s (in %s)", nextBoundary.Format(time.RFC3339), dur.Round(time.Second))

			timer := time.NewTimer(dur)
			select {
			case <-ctx.Done():
				timer.Stop()
				log.Printf("[scheduler] stopped")
				return
			case <-timer.C:
				log.Printf("[scheduler] 09:00 MSK boundary reached, executing daily rollover...")
				if s.onRollover != nil {
					// Use a bounded context for rollover execution
					rolloverCtx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
					s.onRollover(rolloverCtx)
					cancel()
				}
			}
		}
	}()
}
