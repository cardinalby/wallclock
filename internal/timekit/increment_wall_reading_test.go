package timekit

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestIncrementBy(t *testing.T) {
	t.Parallel()

	t.Run("with monotonic clock", func(t *testing.T) {
		tests := []struct {
			name string
			inc  time.Duration
		}{
			{"zero", 0},
			{"positive nanosec", 5 * time.Nanosecond},
			{"positive microsec", 5 * time.Microsecond},
			{"positive millisec", 5 * time.Millisecond},
			{"positive sec", 5 * time.Second},
			{"positive min", 5 * time.Minute},
			{"positive hour", 5 * time.Hour},
			{"negative nanosec", -5 * time.Nanosecond},
			{"negative microsec", -5 * time.Microsecond},
			{"negative millisec", -5 * time.Millisecond},
			{"negative sec", -5 * time.Second},
			{"negative min", -5 * time.Minute},
			{"negative hour", -5 * time.Hour},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
				startTime := time.Now()
				startDiff := GetWallAndMonoClocksDiff(startTime)
				result := IncrementWallReadingBy(startTime, tt.inc)
				require.Equal(t, result.Location(), startTime.Location())
				resultDiff := GetWallAndMonoClocksDiff(result)
				require.Equal(t, startDiff+tt.inc, resultDiff)
				require.True(t, startTime.Equal(result))
				result.Round(0).Equal(startTime.Round(0).Add(tt.inc))
			})
		}
	})

	t.Run("with no monotonic clock", func(t *testing.T) {
		moment := time.Now().Round(0)
		result := IncrementWallReadingBy(moment, time.Second+time.Nanosecond)
		require.Equal(t, result, moment.Add(time.Second+time.Nanosecond))
	})
}
