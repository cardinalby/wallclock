package timekit

import (
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestGetWallAndMonoClocksDiff(t *testing.T) {
	minDiff := time.Duration(math.MaxInt64)
	maxDiff := time.Duration(0)
	diffSum := time.Duration(0)
	measurements := 1000

	for i := 0; i < measurements; i++ {
		time.Sleep(time.Millisecond)
		diff := GetWallAndMonoClocksDiff(time.Now())
		if diff < minDiff {
			minDiff = diff
		}
		if diff > maxDiff {
			maxDiff = diff
		}
		diffSum += diff
	}
	avgDiff := diffSum / time.Duration(measurements)
	t.Logf("Min %v, Max %v, Avg %v", minDiff, maxDiff, avgDiff)
	require.Less(t, minDiff, WallAndMonoClocksDiffThreshold)
}
