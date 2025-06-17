package timekit

import "time"

// WallAndMonoClocksDiffThreshold is the max expected difference between wall clock and monotonic clock
// in normal conditions.
// Go uses 2 separate syscalls to get wall and monotonic clocks,
// so some small difference between them is expected from call to call. According to tests, it's normally
// from nanoseconds to 10 microseconds depending on OS and CPU. I assume the difference more than 1 millisecond
// unlikely to happen, and if it does, it should be considered a wall clock jump.
//
// Given that MinAllowedDelay is 2 ms, we can use the fixed 1ms threshold and compute the check interval
// based on the maxAlarmDelay
const WallAndMonoClocksDiffThreshold = time.Millisecond

var initTime = time.Now()

// GetWallAndMonoClocksDiff says how much wall clock is ahead of the monotonic clock (relative to initTime).
func GetWallAndMonoClocksDiff(t time.Time) time.Duration {
	return t.Round(0).Sub(initTime) - t.Sub(initTime)
}
