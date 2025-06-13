package alarm

import (
	"context"
	"time"
)

var initTime = time.Now()

// getWallAndMonoClocksDiff says how much wall clock is ahead of the monotonic clock (relative to initTime).
func getWallAndMonoClocksDiff(t time.Time) time.Duration {
	return t.Round(0).Sub(initTime) - t.Sub(initTime)
}

// monitorWallClockJumpForward monitors wall time adjustments to the future.
// It calls onJump callback when new wall time > old time + threshold.
// `threshold` is a positive duration that indicates how big jump is acceptable without reporting.
// The function blocks until the context is done.
// It reports individual adjustment events since the last notification, not the total jump
func monitorWallClockJumpForward(
	ctx context.Context,
	checkInterval time.Duration,
	threshold time.Duration,
	onJump func(),
) {
	ticker := time.NewTicker(checkInterval)
	defer ticker.Stop()
	reportIfDiffGtThan := getWallAndMonoClocksDiff(time.Now()) + threshold

	for {
		select {
		case <-ctx.Done():
			return
		case firedAt := <-ticker.C:
			wallMonoDiff := getWallAndMonoClocksDiff(firedAt)
			if wallMonoDiff > reportIfDiffGtThan {
				onJump()
			}
			reportIfDiffGtThan = wallMonoDiff + threshold
		}
	}
}
