package wallclock

import (
	"fmt"
	"time"

	"github.com/cardinalby/wallclock/internal/timekit"
)

type jumpForwardMon struct {
	OnJump        func()
	maxAlarmDelay time.Duration
	done          chan struct{}
}

func (m *jumpForwardMon) Set(maxAlarmDelay time.Duration) {
	if m.done == nil {
		m.maxAlarmDelay = maxAlarmDelay
		m.done = make(chan struct{})
	} else if maxAlarmDelay != m.maxAlarmDelay {
		close(m.done)
		m.done = make(chan struct{})
	} else {
		return
	}
	if isTestingBuild {
		jumpForwardMonitoringGoroutines.Add(1)
		jumpForwardLastMonitoredMaxDelay.Store(int64(maxAlarmDelay))
	}
	go m.run(maxAlarmDelay, m.done)
}

func (m *jumpForwardMon) Stop() {
	if m.done != nil {
		close(m.done)
		m.done = nil
		m.maxAlarmDelay = 0
	}
}

func (m *jumpForwardMon) run(
	maxAlarmDelay time.Duration,
	done chan struct{},
) {
	if isTestingBuild {
		defer func() {
			jumpForwardMonitoringGoroutines.Add(-1)
		}()
	}
	// If I understand correctly, if check_interval + threshold <= maxAlarmDelay
	// monitor can detect wall clock jumps with a precision that is enough to provide requested maxAlarmDelay
	// (given that ticker fires in time).
	// If we use fixed threshold, we can compute the check interval to satisfy the condition above.
	// Another approach would be just take both threshold and check_interval as maxAlarmDelay / 2, but it would
	// lead to more frequent checks and higher CPU usage.
	checkInterval := maxAlarmDelay - timekit.WallAndMonoClocksDiffThreshold
	if checkInterval <= 0 {
		// should not happen because of MinAllowedDelay
		checkInterval = time.Millisecond
		if isTestingBuild {
			panic(fmt.Errorf("maxAlarmDelay is %s", maxAlarmDelay))
		}
	}
	ticker := time.NewTicker(checkInterval)
	defer ticker.Stop()
	startDiff := timekit.GetWallAndMonoClocksDiff(gotTime(time.Now()))

	for {
		select {
		case <-done:
			return
		case firedAt := <-ticker.C:
			wallMonoDiff := timekit.GetWallAndMonoClocksDiff(gotTime(firedAt))
			if wallMonoDiff >= startDiff+timekit.WallAndMonoClocksDiffThreshold {
				m.OnJump()
				// take wallMonoDiff as a new startDiff to detect next jump
				startDiff = wallMonoDiff
			} else if wallMonoDiff < startDiff {
				// If wall clock jumped back, we should not update startDiff so that monitor can detect
				// the next jump forward correctly
				startDiff = wallMonoDiff
			}
			// don't update startDiff in case jump forwards smaller than threshold to be able
			// to detect a series of small jumps
		}
	}
}
