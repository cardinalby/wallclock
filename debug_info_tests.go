//go:build testing

package wallclock

import (
	"time"

	"github.com/cardinalby/wallclock/internal/timekit"
)

const isTestingBuild = true

func gotTime(t time.Time) time.Time {
	return timekit.IncrementWallReadingBy(t, getWallTimeIncrement())
}

func testCleanUp() {
	firedAlarms = nil
	schedulerRunningLoops.Store(0)
	jumpForwardMonitoringGoroutines.Store(0)
	jumpForwardLastMonitoredMaxDelay.Store(0)
	setWallTimeIncrement(0)
	globalScheduler = newScheduler()
}
