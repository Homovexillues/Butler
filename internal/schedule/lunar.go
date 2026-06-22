package schedule

import (
	"time"

	"github.com/6tail/lunar-go/calendar"
)

type LunarSchedule struct {
	Month, Day, Hour, Minute, Second int
}

func (lunar LunarSchedule) NextAfter(since time.Time) (time.Time, bool) {
	nextTime := makeLunarDate(lunar, since.Year())
	if !nextTime.After(since) {
		nextTime = makeLunarDate(lunar, since.Year()+1)
	}
	return nextTime, true
}

func makeLunarDate(lunar LunarSchedule, year int) time.Time {
	lunarDate := calendar.NewLunarFromYmd(year, lunar.Month, lunar.Day)
	solar := lunarDate.GetSolar()
	nextTime := time.Date(solar.GetYear(), time.Month(solar.GetMonth()), solar.GetDay(), lunar.Hour, lunar.Minute, lunar.Second, 0, time.Local)
	if nextTime.Month() != time.Month(solar.GetMonth()) {
		nextTime = nextTime.AddDate(0, 0, -1)
	}
	return nextTime
}
