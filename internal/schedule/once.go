package schedule

import "time"

type Once struct {
	At time.Time
}

func (once Once) NextAfter(since time.Time) (time.Time, bool) {
	if once.At.After(since) {
		return once.At, true
	}
	return time.Time{}, false
}
