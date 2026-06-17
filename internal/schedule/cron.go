package schedule

import (
	"time"

	"github.com/robfig/cron/v3"
)

type CronSchedule struct {
	inner cron.Schedule
}

func NewCronSchedule(spec string) (CronSchedule, error) {
	inner, err := cron.ParseStandard(spec)
	if err != nil {
		return CronSchedule{}, err
	}
	return CronSchedule{inner: inner}, nil
}

func (cronSchedule CronSchedule) NextAfter(since time.Time) (time.Time, bool) {
	next := cronSchedule.inner.Next(since)
	if next.IsZero() {
		return time.Time{}, false
	}
	return next, true
}
