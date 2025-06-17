package timekit

import (
	"time"
	"unsafe"
)

const (
	// Constants from Go's time package
	hasMonotonicBit = 1 << 63
	wallSecondsBits = 33
	wallNanosBits   = 30
	maxWallSeconds  = (1 << wallSecondsBits) - 1 // 33-bit max
	maxWallNanos    = (1 << wallNanosBits) - 1   // 30-bit max (but should be < 1e9)

	// Time epoch differences
	// Go's time package uses Jan 1, year 1 as epoch for ext field
	// But for 33-bit wall seconds, it uses Jan 1, year 1885 as epoch
	year1885Offset = (1885 - 1) * 365.25 * 24 * 3600 // Approximate seconds from year 1 to 1885
)

// timeStruct mirrors the internal structure of time.Time
type timeStruct struct {
	wall uint64
	ext  int64
	loc  *time.Location
}

// IncrementWallReadingBy increments the wall clock time by the given duration
// while preserving the monotonic clock reading
func IncrementWallReadingBy(t time.Time, d time.Duration) time.Time {
	if d == 0 {
		return t
	}
	if t.Round(0) == t {
		// has no monotonic clock reading, just add duration
		return t.Add(d)
	}
	ptr := (*timeStruct)(unsafe.Pointer(&t))
	// Extract current wall seconds (33-bit) and nanoseconds (30-bit)
	wallSecs := (ptr.wall >> wallNanosBits) & maxWallSeconds
	wallNanos := ptr.wall & maxWallNanos

	// Convert duration to seconds and nanoseconds
	dSecs := d / time.Second
	dNanos := d % time.Second

	// Add duration to current time
	totalNanos := int64(wallNanos) + int64(dNanos)
	totalSecs := int64(wallSecs) + int64(dSecs)

	// Handle nanosecond overflow/underflow
	if totalNanos >= 1e9 {
		totalSecs += totalNanos / 1e9
		totalNanos = totalNanos % 1e9
	} else if totalNanos < 0 {
		borrowSecs := (-totalNanos + 1e9 - 1) / 1e9 // Ceiling division
		totalSecs -= borrowSecs
		totalNanos += borrowSecs * 1e9
	}

	// Check if we can still fit in the 33-bit wall seconds field
	if totalSecs >= 0 && totalSecs <= maxWallSeconds {
		// We can still use the compact monotonic format
		ptr.wall = hasMonotonicBit | (uint64(totalSecs) << wallNanosBits) | uint64(totalNanos)
		// ext still contains the monotonic clock reading (unchanged)
	} else {
		// We need to convert to non-monotonic format
		// Calculate absolute seconds since Jan 1, year 1
		absoluteSecs := totalSecs + int64(year1885Offset)

		// Clear the hasMonotonic bit and zero out wall except for nanoseconds
		ptr.wall = uint64(totalNanos)
		ptr.ext = absoluteSecs
	}

	return t
}
