// Package schedule defines the core scheduling abstraction for Butler
package schedule

import (
	"time"
)

type Schedule interface {
	// NextAfter 返回严格晚于 since 的下一次触发时刻。
	// found since时间之后还有没有通知节点
	NextAfter(since time.Time) (next time.Time, found bool)
}
