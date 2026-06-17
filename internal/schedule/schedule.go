// Package schedule defines the core scheduling abstraction for Butler
package schedule

import (
	"time"
)

type Schedule interface {
	// NextAfter 返回严格晚于 since 的下一次触发时刻。
	// ok=false 表示此后再无触发（如已过期的一次性任务）。
	NextAfter(since time.Time) (next time.Time, ok bool)
}
