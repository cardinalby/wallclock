package alarm

import (
	"sync/atomic"
)

var firedAlarms []Alarm
var schedulerRunningLoops atomic.Int64
var jumpForwardMonitoringGoroutines atomic.Int64
var jumpForwardLastMonitoredMaxDelay atomic.Int64
