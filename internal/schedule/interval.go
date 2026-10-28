package schedule

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

type Interval struct {
	every time.Duration
}

func NewIntervalSchedule(expression string) (Interval, error) {
	interval := Interval{}
	for section := range strings.SplitSeq(expression, ",") {
		if len(section) <= 1 {
			return interval, fmt.Errorf("invalid interval section %q: expected a number followed by d, h, m, or s", section)
		}
		numStr := section[:len(section)-1]
		num, err := strconv.Atoi(numStr)
		if err != nil {
			return interval, err
		}
		switch {
		case strings.HasSuffix(section, "d"):
			interval.every += time.Duration(num) * 24 * time.Hour
		case strings.HasSuffix(section, "h"):
			interval.every += time.Duration(num) * time.Hour
		case strings.HasSuffix(section, "m"):
			interval.every += time.Duration(num) * time.Minute
		case strings.HasSuffix(section, "s"):
			interval.every += time.Duration(num) * time.Second
		default:
			return interval, fmt.Errorf("unsupported config character")
		}
	}
	return interval, nil
}

func (interval Interval) NextAfter(since time.Time) (time.Time, bool) {
	if interval.every <= 0 {
		return time.Time{}, false
	} else {
		return since.Add(interval.every), true
	}
}
