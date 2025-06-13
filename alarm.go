package alarm

import (
	"sync/atomic"
	"time"
)

// it's a threshold for detecting wall clock adjustments to the past. If the detected adjustment is greater than
// this threshold, we re-schedule the timer to fire at the correct wall time.
// On real OSes, the timer resolution is 1ms-16ms (sometimes better on MacOS),
// so there is no sense to reschedule the timer in response to smaller adjustments since it will not
// improve the accuracy.
// Some difference between monotonic and wall clocks (relative to another time.Time object) is expected
// (normally smaller than 1 microsecond) because the readings are taken via 2 different system calls.
const monotonicAndWallClocksDiffThreshold = time.Millisecond

// Alarm is an interface that represents a timer that will fire at a specific wall time.
// It is designed to handle wall clock adjustments, ensuring that the alarm fires at the correct wall time
type Alarm interface {
	// C returns a channel that will receive the time when the alarm fires
	C() <-chan time.Time
	// Stop stops the alarm, returns false if the alarm has already expired or been stopped
	Stop() bool
}

type alarm struct {
	wallFireAt time.Time
	c          chan time.Time
	done       chan struct{}
	isDone     atomic.Bool
	factory    *factory
}

func newAlarm(factory *factory, fireAt time.Time) *alarm {
	alarm := &alarm{
		wallFireAt: fireAt.Round(0), // strip monotonic clock component
		c:          make(chan time.Time, 1),
		done:       make(chan struct{}),
		factory:    factory,
	}
	go alarm.start()
	return alarm
}

func newFiredAlarm(firedAt time.Time) *alarm {
	// Create an alarm that has already fired
	alarm := &alarm{
		wallFireAt: firedAt.Round(0),
		c:          make(chan time.Time, 1),
	}
	alarm.c <- firedAt
	alarm.isDone.Store(true)
	return alarm
}

func (a *alarm) start() {
	timer := time.NewTimer(a.wallFireAt.Sub(time.Now()))
	defer timer.Stop()
	defer a.factory.onAlarmIsDone()
	defer a.Stop()

	for {
		select {
		case firedAt := <-timer.C:
			// detect wall clock adjustments to the past. Don't compare it with factory.maxDelay,
			// since we can provide better accuracy in this case by re-scheduling the timer.
			remainingDuration := a.wallFireAt.Sub(firedAt)
			if remainingDuration > monotonicAndWallClocksDiffThreshold {
				timer.Reset(remainingDuration)
			} else {
				// should not block since the channel is buffered and only one value can be sent
				a.c <- firedAt
				return
			}
		case <-a.factory.onWallClockJumpForward.C():
			// wall clock was adjusted to the future
			now := time.Now()
			if !a.wallFireAt.After(now) {
				// alarm is already expired (time jumped over the scheduled wall time), fire it immediately.
				// should not block since the channel is buffered and only one value can be sent
				a.c <- now
				// todo: all expired alarms fire in random order
				return
			}
			// the event is not expired yet but should be rescheduled. Cases are
			// 1. There were no previous jumps to the past: then the timer duration is too long now,
			//    and we should reset it to fire earlier
			// 2. There were jumps to the past:
			//    2.1 the timer already fired, we detected that it's too early and the timer was reset
			//        with a new duration. Now it will fire too late
			//    2.2 the timer hasn't fired, and depending on jumps size it can be too early or too late
			timer.Stop()
			select {
			case <-timer.C:
			default:
			}
			timer.Reset(a.wallFireAt.Sub(time.Now()))
		case <-a.done:
			return
		}
	}
}

func (a *alarm) C() <-chan time.Time {
	return a.c
}

func (a *alarm) Stop() bool {
	if a.isDone.CompareAndSwap(false, true) {
		close(a.done)
		return true
	}
	return false
}
