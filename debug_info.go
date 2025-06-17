package wallclock

import (
	"sync/atomic"
	"time"
)

var firedAlarms []Alarm
var schedulerRunningLoops atomic.Int64
var jumpForwardMonitoringGoroutines atomic.Int64
var jumpForwardLastMonitoredMaxDelay atomic.Int64

// wallTimeIncrement is the duration by which the wall clock of time.Time objects is incremented
// in tests (while monotonic clock stays the same) when the code accesses time.Now() or time returned
// from timers and tickers.
var wallTimeIncrement atomic.Int64

func setWallTimeIncrement(v time.Duration) {
	wallTimeIncrement.Store(int64(v))
}

func getWallTimeIncrement() time.Duration {
	return time.Duration(wallTimeIncrement.Load())
}

func incWallTimeIncrement(v time.Duration) {
	wallTimeIncrement.Add(int64(v))
}
