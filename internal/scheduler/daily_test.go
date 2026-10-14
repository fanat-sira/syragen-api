package scheduler

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestDailySchedulerLifecycle(t *testing.T) {
	loc, _ := time.LoadLocation("Europe/Moscow")

	var wg sync.WaitGroup
	wg.Add(1)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s := NewDailyScheduler(loc, func(rolloverCtx context.Context) {
		// Callback for rollover
	})

	s.Start(ctx)

	// Cancel context to test clean stop
	cancel()
	time.Sleep(50 * time.Millisecond)
}
