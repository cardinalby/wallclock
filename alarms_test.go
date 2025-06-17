package wallclock

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func createTestAlarm(fireAt time.Time, allowedDelay time.Duration) *alarm {
	return &alarm{
		WallFireAt:   fireAt,
		AllowedDelay: allowedDelay,
	}
}

func TestNewAlarms(t *testing.T) {
	t.Parallel()
	alarms := newAlarms()
	require.Equal(t, 0, alarms.Len())
	require.Nil(t, alarms.GetWithSoonestFireAt())
	require.Nil(t, alarms.GetWithSoonestExpTime())
}

func TestAlarms_Add_MultipleAlarms_FireAtOrdering(t *testing.T) {
	t.Parallel()
	alarms := newAlarms()
	now := time.Now()

	// Create alarms with different fire times but same allowed delay
	alarm1 := createTestAlarm(now.Add(time.Hour*3), time.Minute) // fires third
	alarm2 := createTestAlarm(now.Add(time.Hour), time.Minute)   // fires first
	alarm3 := createTestAlarm(now.Add(time.Hour*2), time.Minute) // fires second

	// Add in non-chronological order
	isSoonestFireAt, isSoonestExpTime := alarms.Add(alarm1)
	require.True(t, isSoonestFireAt)
	require.True(t, isSoonestExpTime)
	require.Equal(t, alarm1, alarms.GetWithSoonestFireAt())

	isSoonestFireAt, isSoonestExpTime = alarms.Add(alarm2)
	require.True(t, isSoonestFireAt)  // alarm2 fires earlier
	require.True(t, isSoonestExpTime) // alarm2 expires earlier
	require.Equal(t, alarm2, alarms.GetWithSoonestFireAt())
	require.Equal(t, alarm2, alarms.GetWithSoonestExpTime())

	isSoonestFireAt, isSoonestExpTime = alarms.Add(alarm3)
	require.False(t, isSoonestFireAt)  // alarm2 still fires earliest
	require.False(t, isSoonestExpTime) // alarm2 still expires earliest
	require.Equal(t, alarm2, alarms.GetWithSoonestFireAt())
	require.Equal(t, alarm2, alarms.GetWithSoonestExpTime())

	require.Equal(t, 3, alarms.Len())
}

func TestAlarms_Add_MultipleAlarms_ExpTimeOrdering(t *testing.T) {
	t.Parallel()
	alarms := newAlarms()
	now := time.Now()
	fireAt := now.Add(time.Hour)

	// Create alarms with same fire time but different allowed delays
	alarm1 := createTestAlarm(fireAt, time.Hour)      // expires last
	alarm2 := createTestAlarm(fireAt, time.Minute)    // expires first
	alarm3 := createTestAlarm(fireAt, time.Minute*30) // expires second

	// Add in order of decreasing allowed delay
	isSoonestFireAt, isSoonestExpTime := alarms.Add(alarm1)
	require.True(t, isSoonestFireAt)
	require.True(t, isSoonestExpTime)

	isSoonestFireAt, isSoonestExpTime = alarms.Add(alarm2)
	require.False(t, isSoonestFireAt) // same fire time
	require.True(t, isSoonestExpTime) // expires earlier
	require.Equal(t, alarm2, alarms.GetWithSoonestExpTime())

	isSoonestFireAt, isSoonestExpTime = alarms.Add(alarm3)
	require.False(t, isSoonestFireAt)  // same fire time
	require.False(t, isSoonestExpTime) // alarm2 still expires earliest
	require.Equal(t, alarm2, alarms.GetWithSoonestExpTime())
}

func TestAlarms_Add_ExpTimeOrdering_EqualExpTimeDifferentDelays(t *testing.T) {
	t.Parallel()
	alarms := newAlarms()
	now := time.Now()

	// Create alarms that expire at the same time but have different allowed delays
	// The one with larger allowed delay should be preferred (requires less frequent checks)
	fireAt1 := now.Add(time.Hour)
	fireAt2 := now.Add(time.Hour - time.Minute*10) // fires 10 min earlier
	allowedDelay1 := time.Minute * 30              // smaller delay
	allowedDelay2 := time.Minute * 40              // larger delay
	// Both expire at: now + 1h30m

	alarm1 := createTestAlarm(fireAt1, allowedDelay1)
	alarm2 := createTestAlarm(fireAt2, allowedDelay2)

	// Verify they expire at the same time
	require.True(t, alarm1.wallExpiresAt().Equal(alarm2.wallExpiresAt()))

	isSoonestFireAt, isSoonestExpTime := alarms.Add(alarm1)
	require.True(t, isSoonestFireAt)
	require.True(t, isSoonestExpTime)

	isSoonestFireAt, isSoonestExpTime = alarms.Add(alarm2)
	require.True(t, isSoonestFireAt)  // alarm2 fires earlier
	require.True(t, isSoonestExpTime) // alarm2 has larger allowed delay, so it's preferred
	require.Equal(t, alarm2, alarms.GetWithSoonestFireAt())
	require.Equal(t, alarm2, alarms.GetWithSoonestExpTime())
}

func TestAlarms_Delete_SingleAlarm(t *testing.T) {
	t.Parallel()
	alarms := newAlarms()
	now := time.Now()
	alarm1 := createTestAlarm(now.Add(time.Hour), time.Minute)

	alarms.Add(alarm1)
	require.Equal(t, 1, alarms.Len())

	isDeleted, isSoonestFireAt, isSoonestExpTime := alarms.Delete(alarm1)

	require.True(t, isDeleted)
	require.True(t, isSoonestFireAt)
	require.True(t, isSoonestExpTime)
	require.Equal(t, 0, alarms.Len())
	require.Nil(t, alarms.GetWithSoonestFireAt())
	require.Nil(t, alarms.GetWithSoonestExpTime())
}

func TestAlarms_Delete_MultipleAlarms_DeleteSoonest(t *testing.T) {
	t.Parallel()
	alarms := newAlarms()
	now := time.Now()

	alarm1 := createTestAlarm(now.Add(time.Hour), time.Minute) // soonest
	alarm2 := createTestAlarm(now.Add(time.Hour*2), time.Minute)
	alarm3 := createTestAlarm(now.Add(time.Hour*3), time.Minute)

	alarms.Add(alarm1)
	alarms.Add(alarm2)
	alarms.Add(alarm3)

	// Delete the soonest alarm
	isDeleted, isSoonestFireAt, isSoonestExpTime := alarms.Delete(alarm1)

	require.True(t, isDeleted)
	require.True(t, isSoonestFireAt)
	require.True(t, isSoonestExpTime)
	require.Equal(t, 2, alarms.Len())
	require.Equal(t, alarm2, alarms.GetWithSoonestFireAt())
	require.Equal(t, alarm2, alarms.GetWithSoonestExpTime())
}

func TestAlarms_Delete_MultipleAlarms_DeleteMiddle(t *testing.T) {
	t.Parallel()
	alarms := newAlarms()
	now := time.Now()

	alarm1 := createTestAlarm(now.Add(time.Hour), time.Minute)   // soonest
	alarm2 := createTestAlarm(now.Add(time.Hour*2), time.Minute) // middle
	alarm3 := createTestAlarm(now.Add(time.Hour*3), time.Minute)

	alarms.Add(alarm1)
	alarms.Add(alarm2)
	alarms.Add(alarm3)

	// Delete the middle alarm
	isDeleted, isSoonestFireAt, isSoonestExpTime := alarms.Delete(alarm2)

	require.True(t, isDeleted)
	require.False(t, isSoonestFireAt)  // alarm1 is still soonest
	require.False(t, isSoonestExpTime) // alarm1 is still soonest
	require.Equal(t, 2, alarms.Len())
	require.Equal(t, alarm1, alarms.GetWithSoonestFireAt())
	require.Equal(t, alarm1, alarms.GetWithSoonestExpTime())
}

func TestAlarms_Delete_NonExistentAlarm(t *testing.T) {
	t.Parallel()
	alarms := newAlarms()
	now := time.Now()

	alarm1 := createTestAlarm(now.Add(time.Hour), time.Minute)
	alarm2 := createTestAlarm(now.Add(time.Hour*2), time.Minute) // not added

	alarms.Add(alarm1)

	// Try to delete an alarm that wasn't added
	isDeleted, isSoonestFireAt, isSoonestExpTime := alarms.Delete(alarm2)

	require.False(t, isDeleted)
	require.False(t, isSoonestFireAt)
	require.False(t, isSoonestExpTime)
	require.Equal(t, 1, alarms.Len())
	require.Equal(t, alarm1, alarms.GetWithSoonestFireAt())
}

func TestAlarms_Delete_DifferentExpTimeOrdering(t *testing.T) {
	t.Parallel()
	alarms := newAlarms()
	now := time.Now()

	// Create alarms where fire order != exp time order
	alarm1 := createTestAlarm(now.Add(time.Minute), time.Hour)     // fires first, expires last
	alarm2 := createTestAlarm(now.Add(time.Minute*2), time.Second) // fires second, expires first

	alarms.Add(alarm1)
	alarms.Add(alarm2)

	require.Equal(t, alarm1, alarms.GetWithSoonestFireAt())  // alarm1 fires first
	require.Equal(t, alarm2, alarms.GetWithSoonestExpTime()) // alarm2 expires first

	// Delete alarm1 (soonest fire, but not soonest exp)
	isDeleted, isSoonestFireAt, isSoonestExpTime := alarms.Delete(alarm1)

	require.True(t, isDeleted)
	require.True(t, isSoonestFireAt)   // was soonest fire
	require.False(t, isSoonestExpTime) // was not soonest exp
	require.Equal(t, alarm2, alarms.GetWithSoonestFireAt())
	require.Equal(t, alarm2, alarms.GetWithSoonestExpTime())
}

func TestAlarms_EmptyHeap_Operations(t *testing.T) {
	t.Parallel()
	alarms := newAlarms()

	require.Equal(t, 0, alarms.Len())
	require.Nil(t, alarms.GetWithSoonestFireAt())
	require.Nil(t, alarms.GetWithSoonestExpTime())

	// Delete from empty heap
	now := time.Now()
	alarm1 := createTestAlarm(now.Add(time.Hour), time.Minute)
	isDeleted, isSoonestFireAt, isSoonestExpTime := alarms.Delete(alarm1)

	require.False(t, isDeleted)
	require.False(t, isSoonestFireAt)
	require.False(t, isSoonestExpTime)
}

func TestAlarms_Add_SameAlarmTwice(t *testing.T) {
	t.Parallel()
	alarms := newAlarms()
	now := time.Now()
	alarm1 := createTestAlarm(now.Add(time.Hour), time.Minute)

	// Add first time
	isSoonestFireAt, isSoonestExpTime := alarms.Add(alarm1)
	require.True(t, isSoonestFireAt)
	require.True(t, isSoonestExpTime)
	require.Equal(t, 1, alarms.Len())

	// Add same alarm again
	require.Panics(t, func() {
		alarms.Add(alarm1)
	})
}

func TestAlarms_ZeroAllowedDelay(t *testing.T) {
	t.Parallel()
	alarms := newAlarms()
	now := time.Now()

	// Create alarms with zero allowed delay (WithAnyAllowedDelay)
	alarm1 := createTestAlarm(now.Add(time.Hour), 0)
	alarm2 := createTestAlarm(now.Add(time.Hour*2), 0)

	alarms.Add(alarm1)
	alarms.Add(alarm2)

	require.Equal(t, 2, alarms.Len())
	require.Equal(t, alarm1, alarms.GetWithSoonestFireAt())
	require.True(t, alarms.GetWithSoonestExpTime() == nil)
	alarms.Delete(alarm1)
	require.Equal(t, 1, alarms.Len())
	require.Equal(t, alarm2, alarms.GetWithSoonestFireAt())
	require.True(t, alarms.GetWithSoonestExpTime() == nil)
	alarms.Delete(alarm2)
	require.Equal(t, 0, alarms.Len())
	require.Nil(t, alarms.GetWithSoonestFireAt())
	require.Nil(t, alarms.GetWithSoonestExpTime())
}
