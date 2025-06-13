package alarm

import (
	"context"
	"sync"
	"time"
)

type factory struct {
	monCheckInterval       time.Duration
	monThreshold           time.Duration
	mu                     sync.Mutex
	activeAlarmsCount      int
	onWallClockJumpForward *broadcaster
	cancelWallClockMon     context.CancelFunc
}

type Factory interface {
	NewAlarm(fireAt time.Time) Alarm
}

type factoryProps struct {
	maxDelay time.Duration
}

type FactoryOption func(*factoryProps)

// WithMaxError sets the maximum allowed error for the alarms created using the factory.
// The maxDelay is the maximum allowed difference between the scheduled alarm time and the actual wall clock time
// at which alarm fires. It can't guarantee the exact time (because timers don't have this guarantee), but will
// take care of wall clock adjustments. In the current implementation, the smaller the maxDelay, the more often
// the wall clock adjustments are checked, which may lead to higher CPU usage.
func WithMaxError(maxError time.Duration) FactoryOption {
	return func(props *factoryProps) {
		props.maxDelay = maxError
	}
}

// On real OSes, the timer resolution is 1ms-16ms (sometimes better on MacOS)
const maxErrorLowerBound = time.Millisecond * 2
const DefaultMaxDelay = time.Second

// NewFactory creates a new Factory that can be used to create wall clock Alarms.
func NewFactory(options ...FactoryOption) Factory {
	props := &factoryProps{
		maxDelay: DefaultMaxDelay,
	}
	for _, opt := range options {
		opt(props)
	}
	if props.maxDelay < maxErrorLowerBound {
		props.maxDelay = maxErrorLowerBound
	}
	halfMaxDelay := props.maxDelay / 2

	return &factory{
		onWallClockJumpForward: newBroadcaster(),
		// If I understand correctly, the sum of monCheckInterval and monThreshold should be <= maxDelay
		// to ensure that the monitor can detect adjustments in time
		monCheckInterval: halfMaxDelay,
		monThreshold:     halfMaxDelay,
	}
}

func (f *factory) NewAlarm(fireAt time.Time) Alarm {
	// shortcut: if the fireAt is in the past, we can return a fired alarm immediately
	if now := time.Now(); now.Sub(fireAt) >= 0 {
		return newFiredAlarm(now)
	}

	f.mu.Lock()
	defer f.mu.Unlock()
	if f.activeAlarmsCount == 0 {
		var ctx context.Context
		ctx, f.cancelWallClockMon = context.WithCancel(context.Background())

		go monitorWallClockJumpForward(
			ctx,
			f.monCheckInterval,
			f.monThreshold,
			f.onWallClockJumpForward.Broadcast,
		)
	}
	f.activeAlarmsCount++
	return newAlarm(f, fireAt)
}

func (f *factory) onAlarmIsDone() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.activeAlarmsCount--
	if f.activeAlarmsCount == 0 {
		f.cancelWallClockMon()
		f.cancelWallClockMon = nil
	}
}
