package schedule

import (
	"strconv"
	"strings"
	"time"
)

type Interval struct {
	every time.Duration
}

func NewIntervalSchedule(expression string) (Interval, error) {
	interval := Interval{}
	for _, section := range strings.Split(expression, ",") {
		if len(section) <= 1 {
			return interval, nil
		}
		numStr := section[:len(section)-1] // 或者用你想判断的单位                                                                                                                                                                                                │
		if numStr == "" {
			return interval, nil
		}
		num, err := strconv.Atoi(numStr)
		if err != nil {
			return interval, nil
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
		case section == "":
			continue
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
