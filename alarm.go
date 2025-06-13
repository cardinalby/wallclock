package alarm

import (
	"time"
)

// Alarm is an interface that represents a timer that will fire at a specific wall time.
// It is designed to handle wall clock adjustments, ensuring that the alarm fires at the correct wall time
type Alarm interface {
	// C returns a channel that will receive the time when the alarm fires.
	// Channel is buffered and will receive a value only once
	C() <-chan time.Time

	// Stop stops the alarm, returning false if the alarm has already expired or been stopped
	Stop() bool
}

// NewAlarm creates a new Alarm that will fire not earlier than the specified wall time.
// It may be delayed by DefaultAllowedDelay (or the one specified in options) + unpredictable system timer delay
func NewAlarm(fireAt time.Time, options ...Option) Alarm {
	props := &props{
		allowedDelay: DefaultAllowedDelay,
	}
	for _, opt := range options {
		opt(props)
	}
	if props.allowedDelay < MinAllowedDelay {
		props.allowedDelay = MinAllowedDelay
	}

	alarm := &alarm{
		WallFireAt:   fireAt.Round(0), // strip monotonic clock component
		AllowedDelay: props.allowedDelay,
		c:            make(chan time.Time, 1),
	}
	now := time.Now()
	remainingDuration := alarm.WallFireAt.Sub(now)
	if remainingDuration <= 0 {
		alarm.c <- now
		return alarm
	}
	globalScheduler.AddFutureAlarm(alarm, remainingDuration)
	return alarm
}

type alarm struct {
	WallFireAt   time.Time
	AllowedDelay time.Duration
	c            chan time.Time
}

func (a *alarm) C() <-chan time.Time {
	return a.c
}

func (a *alarm) Stop() bool {
	return globalScheduler.RemoveAlarm(a)
}
