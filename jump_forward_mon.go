package alarm

import (
	"time"
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
	close(m.done)
	m.done = nil
	m.maxAlarmDelay = 0
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
	halfMaxDelay := maxAlarmDelay / 2
	// If I understand correctly, if check_interval + threshold <= maxAlarmDelay
	// monitor can detect wall clock jumps with a precision that is enough to provide requested maxAlarmDelay
	// (given that ticker fires in time)
	threshold := halfMaxDelay
	ticker := time.NewTicker(halfMaxDelay)
	defer ticker.Stop()
	reportIfDiffGtThan := getWallAndMonoClocksDiff(time.Now()) + threshold

	for {
		select {
		case <-done:
			return
		case firedAt := <-ticker.C:
			wallMonoDiff := getWallAndMonoClocksDiff(firedAt)
			if wallMonoDiff > reportIfDiffGtThan {
				m.OnJump()
			}
			reportIfDiffGtThan = wallMonoDiff + threshold
		}
	}
}

var initTime = time.Now()

// getWallAndMonoClocksDiff says how much wall clock is ahead of the monotonic clock (relative to initTime).
func getWallAndMonoClocksDiff(t time.Time) time.Duration {
	return t.Round(0).Sub(initTime) - t.Sub(initTime)
}
