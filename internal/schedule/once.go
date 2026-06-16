package schedule

import "time"

type Once struct {
	At time.Time
}

func (once Once) Next(after time.Time) (time.Time, bool) {
	if once.At.After(after) {
		return once.At, true
	}
	return time.Time{}, false
}
