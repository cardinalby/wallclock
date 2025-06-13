package alarm

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func collectAdjustments(
	t *testing.T,
	collectingDuration time.Duration,
	maxDelay time.Duration,
) (adjCount int) {
	t.Helper()
	mon := &jumpForwardMon{OnJump: func() {
		adjCount++
	}}
	mon.Set(maxDelay)
	<-time.After(collectingDuration)
	mon.Stop()
	time.Sleep(time.Millisecond)
	require.Equal(t, jumpForwardMonitoringGoroutines.Load(), int64(0))
	return adjCount
}

func testAdjustmentsCount(
	t *testing.T,
	testDuration time.Duration,
	expectedAdjustments int,
) {
	t.Helper()
	maxDelays := []time.Duration{
		100 * time.Millisecond,
		time.Second,
	}
	for _, maxDelay := range maxDelays {
		t.Run(fmt.Sprintf("%s maxDelay", maxDelay), func(t *testing.T) {
			t.Parallel()
			adjustments := collectAdjustments(t, testDuration, maxDelay)
			if adjustments != expectedAdjustments {
				t.Errorf("expected %d adjustments, but got %d", expectedAdjustments, adjustments)
			}
		})
	}
}

func TestMonitorWallClockJumpForward(t *testing.T) {
	t.Parallel()

	t.Run("no adjustments", func(t *testing.T) {
		testAdjustmentsCount(t, 3*time.Second, 0)
	})

	t.Run("manual adjustment", func(t *testing.T) {
		// You need to manually adjust the system time to the future while it's running
		t.Skip("manual adjustment test is skipped by default, comment to run it")
		testAdjustmentsCount(t, 15*time.Second, 1)
	})
}
