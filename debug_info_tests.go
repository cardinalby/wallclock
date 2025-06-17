//go:build testing

package wallclock

import (
	"time"

	"github.com/cardinalby/wallclock/internal/wall_clock"
)

const isTestingBuild = true

func gotTime(t time.Time) time.Time {
	return wall_clock.IncrementBy(t, getWallTimeIncrement())
}

func testCleanUp() {
	firedAlarms = nil
	schedulerRunningLoops.Store(0)
	jumpForwardMonitoringGoroutines.Store(0)
	jumpForwardLastMonitoredMaxDelay.Store(0)
	setWallTimeIncrement(0)
	globalScheduler = newScheduler()
}
