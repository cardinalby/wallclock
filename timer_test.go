package wallclock

import (
	"context"
	"math/rand"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestTimerDoesNotFireAfterStop(t *testing.T) {
	t.Parallel()
	// check if timer can put value to its channel after Stop()
	timersCount := 1000
	delay := 100 * time.Millisecond
	timers := make([]*time.Timer, timersCount)
	for i := 0; i < timersCount; i++ {
		timers[i] = time.NewTimer(delay + time.Duration(rand.Intn(10)))
	}
	var isFirstFired atomic.Bool
	var stoppedCount atomic.Int64
	var notStoppedCount atomic.Int64
	var firedCount atomic.Int64
	firstFired := make(chan struct{})
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	wg := &sync.WaitGroup{}
	for _, timer := range timers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			select {
			case <-timer.C:
				firedCount.Add(1)
				if !isFirstFired.Swap(true) {
					close(firstFired)
					return
				}
			case <-firstFired:
				if timer.Stop() {
					stoppedCount.Add(1)
					select {
					case <-timer.C:
						t.Error("Timer channel should not have fired after Stop(), but it did")
					case <-ctx.Done():
						return
					}
				} else {
					notStoppedCount.Add(1)
					select {
					case <-timer.C:
						firedCount.Add(1)
					case <-ctx.Done():
						t.Error("Stop() returned false, but timer channel did not fire")
					}
				}
			}
		}()
	}
	wg.Wait()
	require.EqualValues(t, timersCount, firedCount.Load()+stoppedCount.Load())
	t.Logf("Timers stopped: %d, not stopped: %d, fired: %d",
		stoppedCount.Load(), notStoppedCount.Load(), firedCount.Load())
}

func BenchmarkTimers(b *testing.B) {
	for i := 0; i < b.N; i++ {
		timersCount := 1000
		timers := make([]*time.Timer, timersCount)
		for i := 0; i < timersCount; i++ {
			timers[i] = time.NewTimer(time.Microsecond)
		}
		wg := sync.WaitGroup{}
		for _, timer := range timers {
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-timer.C
			}()
		}
		wg.Wait()
	}
}
