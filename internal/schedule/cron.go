package schedule

import (
	"time"

	"github.com/robfig/cron/v3"
)

type Cron struct {
	inner cron.Schedule
}

func NewCronSchedule(spec string) (Cron, error) {
	inner, err := cron.ParseStandard(spec)
	if err != nil {
		return Cron{}, err
	}
	return Cron{inner: inner}, nil
}

func (cronSchedule Cron) NextAfter(since time.Time) (time.Time, bool) {
	next := cronSchedule.inner.Next(since)
	if next.IsZero() {
		return time.Time{}, false
	}
	return next, true
}
