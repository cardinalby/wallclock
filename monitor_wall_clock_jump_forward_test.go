package alarm

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func collectAdjustments(
	t *testing.T,
	collectingDuration time.Duration,
	reportThreshold time.Duration,
	checkInterval time.Duration,
) (adjCount int) {
	t.Helper()
	startedAt := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), collectingDuration)
	defer cancel()
	doneTimeExpired := time.After(collectingDuration + 500*time.Millisecond)
	monitorDone := make(chan struct{})

	go func() {
		monitorWallClockJumpForward(ctx, checkInterval, reportThreshold, func() {
			adjCount++
		})
		close(monitorDone)
	}()

	select {
	case <-monitorDone:
		if time.Since(startedAt) < collectingDuration {
			t.Errorf("monitorWallClockJumpForward completed early")
		}
		return adjCount

	case <-doneTimeExpired:
		t.Error("monitorWallClockJumpForward did not complete within the expected time")
		return 0
	}
}

func testAdjustmentsCount(
	t *testing.T,
	testDuration time.Duration,
	expectedAdjustments int,
) {
	t.Helper()
	reportThreshold := time.Millisecond
	checkIntervals := []time.Duration{
		//time.Microsecond,
		//10 * time.Microsecond,
		//100 * time.Microsecond,
		//time.Millisecond,
		//10 * time.Millisecond,
		100 * time.Millisecond,
	}
	for _, checkInterval := range checkIntervals {
		t.Run(fmt.Sprintf("%s check interval", checkInterval), func(t *testing.T) {
			t.Parallel()
			adjustments := collectAdjustments(t, testDuration, reportThreshold, checkInterval)
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
