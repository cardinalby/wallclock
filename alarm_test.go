package alarm

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func hasMonoClockReading(moment time.Time) bool {
	return moment.Round(0) != moment
}

func TestHasMonoClockReading(t *testing.T) {
	t.Parallel()
	now := time.Now()
	assert.True(t, hasMonoClockReading(now))
	assert.False(t, hasMonoClockReading(now.Round(0)))
}

func requireMonoClockReading(t *testing.T, moment time.Time) {
	t.Helper()
	assert.True(t, hasMonoClockReading(moment))
}

func TestFactory_NewAlarmWithNoAdjustments(t *testing.T) {
	t.Parallel()
	f := NewFactory(WithMaxError(time.Millisecond * 500))
	fImpl := f.(*factory)
	assert.Equal(t, 250*time.Millisecond, fImpl.monCheckInterval)
	assert.Equal(t, 250*time.Millisecond, fImpl.monThreshold)
	now := time.Now()
	alarmInPast := f.NewAlarm(now.Add(-time.Hour))
	alarm1ms := f.NewAlarm(now.Add(time.Millisecond))
	alarm100ms := f.NewAlarm(now.Add(time.Millisecond * 100))
	alarm500ms := f.NewAlarm(now.Add(time.Millisecond * 500))
	alarm800ms := f.NewAlarm(now.Add(time.Millisecond * 800))
	alarm1s := f.NewAlarm(now.Add(time.Second))

	assert.Equal(t, 5, fImpl.activeAlarmsCount)

	done := make(chan struct{})
	takesTooLong := time.After(time.Millisecond * 1500)

	var firedAlarms []Alarm
	addFiredAlarm := func(name string, a Alarm, firedAt time.Time, expFiredAt time.Time) {
		t.Helper()
		requireMonoClockReading(t, firedAt)
		aImpl := a.(*alarm)
		if name != "alarmInPast" {
			assert.True(t,
				expFiredAt.Equal(aImpl.wallFireAt),
				"name: %s, expFiredAt: %s, aImpl.wallFireAt: %s",
				name, expFiredAt, aImpl.wallFireAt,
			)
		}
		if firedAt.Before(expFiredAt) {
			t.Errorf("%s fired too soon at %s, but expected to fire at %s", name, firedAt, expFiredAt)
		} else if firedAt.After(expFiredAt.Add(time.Millisecond * 100)) {
			t.Errorf("%s fired too late at %s, but expected to fire at %s", name, firedAt, expFiredAt)
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
			assert.True(t, alarm800ms.Stop())
		case firedAt := <-alarm800ms.C():
			t.Errorf("alarm800ms should have been stopped, but it fired at %s", firedAt)
		case firedAt := <-alarm1s.C():
			addFiredAlarm("alarm1s", alarm1s, firedAt, now.Add(time.Second))
		case <-takesTooLong:
			t.Errorf("All alarms did not fire in expected time")
		case <-done:
			loop = false
		}
	}
	time.Sleep(time.Millisecond * 100) // give some time for the alarms to stop
	assert.Equal(
		t,
		[]Alarm{alarmInPast, alarm1ms, alarm100ms, alarm500ms, alarm1s},
		firedAlarms,
	)
	assert.Nil(t, fImpl.cancelWallClockMon)
	assert.Equal(t, 0, fImpl.activeAlarmsCount)
	assert.False(t, alarmInPast.Stop())
	assert.False(t, alarm1ms.Stop())
	assert.False(t, alarm100ms.Stop())
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

func TestFactory_NewAlarmHasBufferedChan(t *testing.T) {
	f := NewFactory()
	fImpl := f.(*factory)
	assert.Equal(t, DefaultMaxDelay/2, fImpl.monThreshold)
	assert.Equal(t, DefaultMaxDelay/2, fImpl.monCheckInterval)
	now := time.Now()
	a := f.NewAlarm(now.Add(time.Hour))
	assert.Equal(t, 1, fImpl.activeAlarmsCount)
	assert.True(t, a.Stop())
	time.Sleep(time.Millisecond * 100) // give some time for the alarm to stop
	assert.Equal(t, 0, fImpl.activeAlarmsCount)
	select {
	case <-a.C():
		t.Error("Alarm channel should not have fired, but it did")
	default:
	}
}

func TestFactory_NewAlarmConcurrentCreation(t *testing.T) {
	t.Parallel()
	f := NewFactory(WithMaxError(time.Millisecond * 500))
	fImpl := f.(*factory)
	alarmsCount := 1000
	alarms := make([]Alarm, 0, alarmsCount)
	alarmsChan := make(chan Alarm, alarmsCount)
	fireAt := time.Now().Add(time.Millisecond * 200)

	for i := 0; i < alarmsCount; i++ {
		go func() {
			alarmsChan <- f.NewAlarm(fireAt)
		}()
	}
	for a := range alarmsChan {
		alarms = append(alarms, a)
		if len(alarms) == alarmsCount {
			close(alarmsChan)
			break
		}
	}
	assert.Equal(t, alarmsCount, len(alarms))
	assert.Equal(t, alarmsCount, fImpl.activeAlarmsCount)
	time.Sleep(time.Millisecond * 500)
	for i := 0; i < alarmsCount; i++ {
		select {
		case <-alarms[i].C():
		default:
			t.Errorf("Alarm %d has not fired", i)
		}
	}
	assert.Equal(t, 0, fImpl.activeAlarmsCount)
}

func TestFactory_NewAlarmConcurrentStop(t *testing.T) {
	for i := 0; i < 5; i++ {
		wg := &sync.WaitGroup{}
		f := NewFactory(WithMaxError(time.Millisecond * 500))
		fImpl := f.(*factory)
		a := f.NewAlarm(time.Now().Add(time.Hour))
		var stoppedTimes atomic.Int64
		assert.Equal(t, 1, fImpl.activeAlarmsCount)
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
		assert.Equal(t, int64(1), stoppedTimes.Load())
		assert.Equal(t, 0, fImpl.activeAlarmsCount)
		assert.Nil(t, fImpl.cancelWallClockMon)
	}
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

	maxErr := time.Millisecond * 500
	f := NewFactory(WithMaxError(maxErr))
	fImpl := f.(*factory)
	now := time.Now()
	fireA1At := now.Add(time.Minute)
	fireA2At := now.Add(time.Hour)
	a1 := f.NewAlarm(fireA1At)
	t.Logf("a1 scheduled to fire at %s", fireA1At)
	a2 := f.NewAlarm(fireA2At)
	t.Logf("a2 scheduled to fire at %s", fireA2At)
	assert.Equal(t, 2, fImpl.activeAlarmsCount)
	var firedAlarms []Alarm
	for i := 0; i < 2; i++ {
		select {
		case firedAt := <-a1.C():
			firedAlarms = append(firedAlarms, a1)
			t.Logf("a1 fired at %s", firedAt)
			if firedAt.Round(0).Before(fireA1At.Add(-maxErr)) {
				t.Errorf("a1 fired too soon at %s, expected at least %s",
					firedAt.Round(0), fireA1At.Add(-maxErr).Round(0),
				)
			}
		case firedAt := <-a2.C():
			firedAlarms = append(firedAlarms, a2)
			t.Logf("Alarm a2 fired at %s", firedAt)
			if firedAt.Round(0).Before(fireA2At.Add(-maxErr)) {
				t.Errorf("a2 fired too soon at %s, expected at least %s",
					firedAt.Round(0), fireA2At.Add(-maxErr).Round(0),
				)
			}
		}
	}
	assert.Equal(t, []Alarm{a1, a2}, firedAlarms)
	assert.Nil(t, fImpl.cancelWallClockMon)
	assert.Equal(t, 0, fImpl.activeAlarmsCount)
}

func TestFactory_NewAlarmWithManualAdjustmentToPast(t *testing.T) {
	t.Parallel()
	// You need to manually adjust the system time to the past a bit while it's running
	// We expect alarm should fire at the wall time it was scheduled, not sooner
	t.Skip("manual adjustment test is skipped by default, comment to run it")

	maxErr := time.Millisecond * 500
	f := NewFactory(WithMaxError(maxErr))
	fImpl := f.(*factory)
	now := time.Now()
	fireA1At := now.Add(time.Second * 10)
	fireA2At := now.Add(time.Second * 15)
	a1 := f.NewAlarm(fireA1At)
	t.Logf("a1 scheduled to fire at %s", fireA1At)
	a2 := f.NewAlarm(fireA2At)
	t.Logf("a2 scheduled to fire at %s", fireA2At)
	assert.Equal(t, 2, fImpl.activeAlarmsCount)
	var firedAlarms []Alarm
	for i := 0; i < 2; i++ {
		select {
		case firedAt := <-a1.C():
			firedAlarms = append(firedAlarms, a1)
			elapsed := time.Since(now)
			t.Logf("a1 fired at %s (%v after start)", firedAt, elapsed)
			checkTimeEqualsWithErr(t, "a1", fireA1At, firedAt.Round(0), maxErr)
		case firedAt := <-a2.C():
			firedAlarms = append(firedAlarms, a2)
			t.Logf("a2 fired at %s (%v  after start)", firedAt, time.Since(now))
			checkTimeEqualsWithErr(t, "a2", fireA2At, firedAt.Round(0), maxErr)
		}
	}
	assert.Equal(t, []Alarm{a1, a2}, firedAlarms)
	assert.Nil(t, fImpl.cancelWallClockMon)
	assert.Equal(t, 0, fImpl.activeAlarmsCount)
}
