// Package schedule defines the core scheduling abstraction for Butler
package schedule

import (
	"time"
)

type Schedule interface {
	// judge if should notice after the "after" time
	Next(after time.Time) (time.Time, bool)
}
