//go:build !testing

package wallclock

import "time"

const isTestingBuild = false

func gotTime(t time.Time) time.Time {
	return t
}

func testCleanUp() {}
