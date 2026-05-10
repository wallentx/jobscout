package health

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestRunThrottledHealthStepLimitsConcurrency(t *testing.T) {
	sem := make(chan struct{}, 1)
	var active atomic.Int32
	var maxActive atomic.Int32

	var wg sync.WaitGroup
	for range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := runThrottledHealthStep(context.Background(), sem, "test", "Acme", func() error {
				now := active.Add(1)
				for {
					prev := maxActive.Load()
					if now <= prev || maxActive.CompareAndSwap(prev, now) {
						break
					}
				}
				time.Sleep(10 * time.Millisecond)
				active.Add(-1)
				return nil
			})
			if err != nil {
				t.Errorf("runThrottledHealthStep() error = %v, want nil", err)
			}
		}()
	}
	wg.Wait()

	if got := maxActive.Load(); got != 1 {
		t.Fatalf("max concurrent executions = %d; want 1", got)
	}
}
