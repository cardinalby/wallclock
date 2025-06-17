package wallclock

import "time"

type props struct {
	allowedDelay time.Duration
}

type Option func(*props)

// DefaultAllowedDelay is the default maximum allowed delay for the alarm if no options is provided.
const DefaultAllowedDelay = time.Second

// MinAllowedDelay is the minimum allowed delay for the alarm.
// On real OSes, the timer resolution is 1ms-16ms (sometimes better on MacOS)
const MinAllowedDelay = time.Millisecond * 2

// WithAllowedDelay sets the maximum allowed delay for the alarm.
// The allowedDelay is the maximum allowed difference between the scheduled alarm wall time and the actual
// wall clock time at which alarm fires given that internal timer fires just in time (which doesn't happen).
// The lib takes care of wall clock adjustments only.
// In the current implementation, the smaller the allowedDelay, the more often the wall clock adjustments are
// checked, which may lead to higher CPU usage
// It's limited by MinAllowedDelay
func WithAllowedDelay(allowedDelay time.Duration) Option {
	if allowedDelay < MinAllowedDelay {
		allowedDelay = MinAllowedDelay
	}
	return func(props *props) {
		props.allowedDelay = allowedDelay
	}
}

// WithAnyAllowedDelay disables wall clock jump forward monitoring for the alarm. Jumps to the past will
// still be handled.
func WithAnyAllowedDelay() Option {
	return func(props *props) {
		props.allowedDelay = 0
	}
}
