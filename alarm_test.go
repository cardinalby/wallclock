package alarm

import (
	"context"
	"math/rand"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func hasMonoClockReading(moment time.Time) bool {
	return moment.Round(0) != moment
}

func TestHasMonoClockReading(t *testing.T) {
	t.Parallel()
	now := time.Now()
	require.True(t, hasMonoClockReading(now))
	require.False(t, hasMonoClockReading(now.Round(0)))
}

func requireMonoClockReading(t *testing.T, moment time.Time) {
	t.Helper()
	require.True(t, hasMonoClockReading(moment))
}

func requireSchedulerState(t *testing.T, alarmsCount int, jumpForwardMonLastMaxDelay time.Duration) {
	t.Helper()
	globalScheduler.mu.Lock()
	require.Equal(t, alarmsCount, globalScheduler.heap.Len())
	globalScheduler.mu.Unlock()
	if alarmsCount > 0 {
		require.EqualValues(t, 1, schedulerRunningLoops.Load())
		require.EqualValues(t, jumpForwardMonLastMaxDelay, jumpForwardLastMonitoredMaxDelay.Load())
		require.EqualValues(t, 1, jumpForwardMonitoringGoroutines.Load())
	} else {
		require.EqualValues(t, 0, schedulerRunningLoops.Load())
		require.EqualValues(t, jumpForwardMonLastMaxDelay, jumpForwardLastMonitoredMaxDelay.Load())
	}
}

func Test_NewAlarmWithNoAdjustments(t *testing.T) {
	allowedDelay := time.Millisecond * 500
	allowedDelayOpt := WithAllowedDelay(allowedDelay)
	now := time.Now()
	alarmInPast := NewAlarm(now.Add(-time.Hour), allowedDelayOpt)
	alarm1ms := NewAlarm(now.Add(time.Millisecond), allowedDelayOpt)
	alarm100ms := NewAlarm(now.Add(time.Millisecond*100), allowedDelayOpt)
	alarm500ms := NewAlarm(now.Add(time.Millisecond*500), allowedDelayOpt)
	alarm800ms := NewAlarm(now.Add(time.Millisecond*800), allowedDelayOpt)
	alarm1s := NewAlarm(now.Add(time.Second), allowedDelayOpt)
	requireSchedulerState(t, 5, allowedDelay)

	done := make(chan struct{})
	takesTooLong := time.After(time.Millisecond * 1500)

	var firedAlarms []Alarm
	addFiredAlarm := func(name string, a Alarm, firedAt time.Time, expFiredAt time.Time) {
		t.Helper()
		requireMonoClockReading(t, firedAt)
		aImpl := a.(*alarm)
		if name != "alarmInPast" {
			require.True(t,
				expFiredAt.Equal(aImpl.WallFireAt),
				"name: %s, expFiredAt: %s, aImpl.WallFireAt: %s",
				name, expFiredAt, aImpl.WallFireAt,
			)
		}
		if firedAt.Before(expFiredAt) {
			t.Errorf("%s fired too soon at %s, but expected to Fire at %s", name, firedAt, expFiredAt)
		} else if firedAt.After(expFiredAt.Add(time.Millisecond * 100)) {
			t.Errorf("%s fired too late at %s, but expected to Fire at %s", name, firedAt, expFiredAt)
		}
		firedAlarms = append(firedAlarms, a)
		if len(firedAlarms) == 5 {
			close(done)
		}
	}

	for loop := true; loop; {
		select {
		case firedAt := <-alarmInPast.C():
			addFiredAlarm("alarmInPast", alarmInPast, firedAt, now)
		case firedAt := <-alarm1ms.C():
			addFiredAlarm("alarm1ms", alarm1ms, firedAt, now.Add(time.Millisecond))
		case firedAt := <-alarm100ms.C():
			addFiredAlarm("alarm100ms", alarm100ms, firedAt, now.Add(time.Millisecond*100))
		case firedAt := <-alarm500ms.C():
			addFiredAlarm("alarm500ms", alarm500ms, firedAt, now.Add(time.Millisecond*500))
			require.True(t, alarm800ms.Stop())
		case firedAt := <-alarm800ms.C():
			t.Errorf("alarm800ms should have been stopped, but it fired at %s", firedAt)
		case firedAt := <-alarm1s.C():
			addFiredAlarm("alarm1s", alarm1s, firedAt, now.Add(time.Second))
		case <-takesTooLong:
			t.Errorf("All alarms did not Fire in expected time")
		case <-done:
			loop = false
		}
	}
	time.Sleep(time.Millisecond * 100) // give some time for the alarms to stop
	require.Equal(
		t,
		[]Alarm{alarmInPast, alarm1ms, alarm100ms, alarm500ms, alarm1s},
		firedAlarms,
	)
	requireSchedulerState(t, 0, allowedDelay)
	require.False(t, alarmInPast.Stop())
	require.False(t, alarm1ms.Stop())
	require.False(t, alarm100ms.Stop())
	errNotStopped := func(name string) {
		t.Helper()
		t.Errorf("alarm %s should have been stopped and chan not closed", name)
	}
	select {
	case <-alarmInPast.C():
		errNotStopped("alarmInPast")
	case <-alarm1ms.C():
		errNotStopped("alarm1ms")
	case <-alarm100ms.C():
		errNotStopped("alarm100ms")
	case <-alarm500ms.C():
		errNotStopped("alarm500ms")
	case <-alarm800ms.C():
		errNotStopped("alarm800ms")
	case <-alarm1s.C():
		errNotStopped("alarm1s")
	default:
	}
}

func TestNewAlarm_Stop(t *testing.T) {
	now := time.Now()
	a := NewAlarm(now.Add(time.Hour))
	requireSchedulerState(t, 1, DefaultAllowedDelay)
	require.True(t, a.Stop())
	time.Sleep(time.Millisecond * 100) // give some time for the alarm to stop
	requireSchedulerState(t, 0, DefaultAllowedDelay)
	select {
	case <-a.C():
		t.Error("Alarm channel should not have fired, but it did")
	default:
	}
}

func TestNewAlarm_ConcurrentCreationAndFire(t *testing.T) {
	allowedDelay := time.Millisecond * 500
	allowedDelayOpt := WithAllowedDelay(allowedDelay)
	alarmsCount := 1000
	alarms := make([]Alarm, 0, alarmsCount)
	alarmsChan := make(chan Alarm, alarmsCount)
	fireAt := time.Now().Add(time.Millisecond * 200)

	for i := 0; i < alarmsCount; i++ {
		go func() {
			alarmsChan <- NewAlarm(fireAt, allowedDelayOpt)
		}()
	}
	for a := range alarmsChan {
		alarms = append(alarms, a)
		if len(alarms) == alarmsCount {
			close(alarmsChan)
			break
		}
	}
	require.Equal(t, alarmsCount, len(alarms))
	requireSchedulerState(t, alarmsCount, allowedDelay)
	time.Sleep(time.Millisecond * 500)
	for i := 0; i < alarmsCount; i++ {
		select {
		case <-alarms[i].C():
		default:
			t.Errorf("Alarm %d has not fired", i)
		}
	}
	for _, a := range alarms {
		require.False(t, a.Stop())
	}
	requireSchedulerState(t, 0, allowedDelay)
}

func TestNewAlarm_ConcurrentStop(t *testing.T) {
	for i := 0; i < 5; i++ {
		wg := &sync.WaitGroup{}
		a := NewAlarm(time.Now().Add(time.Hour))
		var stoppedTimes atomic.Int64
		requireSchedulerState(t, 1, DefaultAllowedDelay)
		for j := 0; j < 20; j++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				if a.Stop() {
					stoppedTimes.Add(1)
				}
			}()
		}
		wg.Wait()
		time.Sleep(time.Millisecond * 100) // give some time for the alarm to stop
		require.Equal(t, int64(1), stoppedTimes.Load())
		requireSchedulerState(t, 0, DefaultAllowedDelay)
	}
}

func TestNewAlarm_DoesNotFireAfterStop(t *testing.T) {
	// check if alarm puts a value to its channel after Stop()
	alarmsCount := 1000
	delay := 100 * time.Millisecond
	alarms := make([]Alarm, alarmsCount)
	baseTime := time.Now().Add(delay)
	for i := 0; i < alarmsCount; i++ {
		alarms[i] = NewAlarm(baseTime.Add(time.Duration(rand.Intn(10))))
	}
	requireSchedulerState(t, alarmsCount, DefaultAllowedDelay)
	var isFirstFired atomic.Bool
	var stoppedCount atomic.Int64
	var notStoppedCount atomic.Int64
	var firedCount atomic.Int64
	firstFired := make(chan struct{})
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	wg := &sync.WaitGroup{}
	for i, alarm := range alarms {
		wg.Add(1)
		go func() {
			defer wg.Done()
			select {
			case firedAt := <-alarm.C():
				firedCount.Add(1)
				requireMonoClockReading(t, firedAt)
				checkTimeEqualsWithErr(t, "alarm"+strconv.Itoa(i), time.Now(), firedAt, time.Millisecond*200)
				if !isFirstFired.Swap(true) {
					close(firstFired)
					return
				}
			case <-firstFired:
				if alarm.Stop() {
					stoppedCount.Add(1)
					select {
					case <-alarm.C():
						t.Error("Alarm fired after Stop()")
					case <-ctx.Done():
						return
					}
				} else {
					notStoppedCount.Add(1)
					select {
					case <-alarm.C():
						firedCount.Add(1)
					case <-ctx.Done():
						t.Error("Stop() returned false, but alarm channel did not fire")
					}
				}
			}
		}()
	}
	wg.Wait()
	requireSchedulerState(t, 0, DefaultAllowedDelay)
	t.Logf("Alarms stopped: %d not stopped: %d, fired: %d",
		stoppedCount.Load(), notStoppedCount.Load(), firedCount.Load(),
	)
	require.EqualValues(t, alarmsCount, firedCount.Load()+stoppedCount.Load())

}

func checkTimeEqualsWithErr(t *testing.T, name string, expected, actual time.Time, maxError time.Duration) {
	t.Helper()
	actual = actual.Round(0)     // strip monotonic clock component
	expected = expected.Round(0) // strip monotonic clock component
	if actual.Before(expected.Add(-maxError)) || actual.After(expected.Add(maxError)) {
		t.Errorf(
			"Expected time %s from %s is not equal to actual time %s with max error %s",
			expected, name, actual, maxError,
		)
	}
}

func TestFactory_NewAlarmWithManualAdjustmentToFuture(t *testing.T) {
	// You need to manually adjust the system time to the future by 1 hour while it's running
	// We expect alarm should fire sooner than internal timer was initially scheduled
	t.Skip("manual adjustment test is skipped by default, comment to run it")

	maxDelay := time.Millisecond * 500
	maxDelayOpt := WithAllowedDelay(maxDelay)
	now := time.Now()
	baseTime := now.Add(time.Minute)
	var alarms []Alarm
	alarmsCount := 20
	for i := 0; i < alarmsCount; i++ {
		fireAt := baseTime.Add(time.Duration(i) * time.Second)
		alarms = append(alarms, NewAlarm(fireAt, maxDelayOpt))
	}
	requireSchedulerState(t, alarmsCount, maxDelay)
	for i := 0; i < alarmsCount; i++ {
		firedAt := <-alarms[i].C()
		requireMonoClockReading(t, firedAt)
		checkTimeEqualsWithErr(t, "alarm"+strconv.Itoa(i), time.Now(), firedAt.Round(0), 100*time.Millisecond)
	}
	requireSchedulerState(t, 0, maxDelay)
	require.Equal(t, firedAlarms, alarms)
}

func TestFactory_NewAlarmWithManualAdjustmentToPast(t *testing.T) {
	// You need to manually adjust the system time to the past a bit while it's running
	// We expect alarm should fire at the wall time it was scheduled, not sooner
	t.Skip("manual adjustment test is skipped by default, comment to run it")

	now := time.Now()
	fireA1At := now.Add(time.Second * 10)
	fireA2At := now.Add(time.Second * 15)
	a1 := NewAlarm(fireA1At)
	t.Logf("a1 scheduled to fire at %s", fireA1At)
	a2 := NewAlarm(fireA2At)
	t.Logf("a2 scheduled to fire at %s", fireA2At)
	requireSchedulerState(t, 2, DefaultAllowedDelay)
	var fired []Alarm
	for i := 0; i < 2; i++ {
		select {
		case firedAt := <-a1.C():
			fired = append(fired, a1)
			elapsed := time.Since(now)
			t.Logf("a1 fired at %s (%v after start)", firedAt, elapsed)
			checkTimeEqualsWithErr(t, "a1", fireA1At, firedAt.Round(0), time.Millisecond*100)
		case firedAt := <-a2.C():
			fired = append(fired, a2)
			t.Logf("a2 fired at %s (%v  after start)", firedAt, time.Since(now))
			checkTimeEqualsWithErr(t, "a2", fireA2At, firedAt.Round(0), time.Millisecond*100)
		}
	}
	require.Equal(t, []Alarm{a1, a2}, fired)
	require.Equal(t, fired, firedAlarms)
	requireSchedulerState(t, 0, DefaultAllowedDelay)
}

func BenchmarkAlarms(b *testing.B) {
	for i := 0; i < b.N; i++ {
		alarmsCount := 1000
		alarms := make([]Alarm, alarmsCount)
		fireAt := time.Now().Add(time.Microsecond)
		for i := 0; i < alarmsCount; i++ {
			alarms[i] = NewAlarm(fireAt)
		}
		wg := sync.WaitGroup{}
		for _, alarm := range alarms {
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-alarm.C()
			}()
		}
		wg.Wait()
	}
}
