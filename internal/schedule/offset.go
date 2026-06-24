package schedule

import (
	"fmt"
	"time"
)

type OffsetSchedule struct {
	base    Schedule
	offsets []time.Duration
}

func NewOffsetSchedule(base Schedule, offsets []time.Duration) (OffsetSchedule, error) {
	if len(offsets) <= 0 {
		return OffsetSchedule{}, fmt.Errorf("offsets empty")
	}
	return OffsetSchedule{
		base:    base,
		offsets: offsets,
	}, nil
}

func (offsetSchedule OffsetSchedule) NextAfter(since time.Time) (time.Time, bool) {
	var earliest time.Time
	offsetFound := false
	for _, offset := range offsetSchedule.offsets {
		// 这里算的是通知时间，要减去偏移量的干扰
		next, baseFound := offsetSchedule.base.NextAfter(since.Add(-offset))
		// base里是否有有效的时间节点
		if !baseFound {
			continue
		}
		// base里既然已经找到了，说明一定会有合法通知
		// 这里算的是触发时间，所以要加上偏移量时间
		next = next.Add(offset)
		if !offsetFound || next.Before(earliest) {
			earliest, offsetFound = next, true
		}

	}
	return earliest, offsetFound
}
