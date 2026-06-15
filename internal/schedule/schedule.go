// Package schedule defines the core scheduling abstraction for Butler
package schedule

import (
	"time"
)

type Schedule interface {
	Next(after time.Time) (time.Time, bool)
}
