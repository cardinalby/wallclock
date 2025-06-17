package wallclock

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func collectAdjustments(
	t *testing.T,
	stop chan struct{},
	maxDelay time.Duration,
	onMonStarted func(),
) (adjCount int) {
	t.Helper()
	mon := &jumpForwardMon{OnJump: func() {
		adjCount++
	}}
	mon.Set(maxDelay)
	if onMonStarted != nil {
		onMonStarted()
	}
	<-stop
	mon.Stop()
	time.Sleep(time.Millisecond)
	require.Equal(t, jumpForwardMonitoringGoroutines.Load(), int64(0))
	return adjCount
}

func testAdjustmentsCount(
	t *testing.T,
	expectedAdjustments int,
	onMonStarted func(maxDelay time.Duration),
) {
	t.Helper()
	maxDelays := []time.Duration{
		50 * time.Millisecond,
		100 * time.Millisecond,
	}
	for _, maxDelay := range maxDelays {
		t.Run(fmt.Sprintf("%s maxDelay", maxDelay), func(t *testing.T) {
			onTestStart(t)
			stop := make(chan struct{})
			onMonStartedCallback := func() {
				go func() {
					if onMonStarted != nil {
						onMonStarted(maxDelay)
					}
					close(stop)
				}()
			}
			adjustments := collectAdjustments(t, stop, maxDelay, onMonStartedCallback)
			if adjustments != expectedAdjustments {
				t.Errorf("expected %d adjustments, but got %d", expectedAdjustments, adjustments)
			}
		})
	}
}

func TestJumpForwardMon(t *testing.T) {
	t.Run("no adjustments", func(t *testing.T) {
		testAdjustmentsCount(t, 0, nil)
	})

	t.Run("has adjustment", func(t *testing.T) {
		testAdjustmentsCount(t, 1, func(maxDelay time.Duration) {
			time.Sleep(maxDelay + time.Millisecond)
			setWallTimeIncrement(maxDelay + 1)
			time.Sleep(maxDelay + time.Millisecond)
		})
	})

	t.Run("has 2 adjustments", func(t *testing.T) {
		testAdjustmentsCount(t, 2, func(maxDelay time.Duration) {
			time.Sleep(maxDelay + time.Millisecond*50)
			incWallTimeIncrement(maxDelay + 1)
			time.Sleep(maxDelay + time.Millisecond*50)
			incWallTimeIncrement(maxDelay + 1)
			time.Sleep(maxDelay + time.Millisecond*50)
		})
	})

	t.Run("has adjustment to future and past", func(t *testing.T) {
		testAdjustmentsCount(t, 1, func(maxDelay time.Duration) {
			time.Sleep(maxDelay + time.Millisecond*50)
			setWallTimeIncrement(-maxDelay * 100)
			time.Sleep(maxDelay + time.Millisecond*50)
			incWallTimeIncrement(maxDelay + 1)
			time.Sleep(maxDelay + time.Millisecond*50)
		})
	})

	t.Run("has multiple small adjustments to future", func(t *testing.T) {
		testAdjustmentsCount(t, 3, func(maxDelay time.Duration) {
			time.Sleep(maxDelay + time.Millisecond*50)
			incWallTimeIncrement(maxDelay/3 + 1)
			time.Sleep(maxDelay + time.Millisecond*50)
			incWallTimeIncrement(maxDelay/3 + 1)
			time.Sleep(maxDelay + time.Millisecond*50)
			incWallTimeIncrement(maxDelay/3 + 1)
			time.Sleep(maxDelay + time.Millisecond*50)
		})
	})
}

func TestWithAnyAllowedDelay(t *testing.T) {
	fireAt, _ := time.Parse(time.RFC3339, "2026-01-01T09:00:00Z")
	t.Logf("fireAt: %s", fireAt)
}
