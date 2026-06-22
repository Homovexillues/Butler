package schedule

import "time"

type SolarAnnual struct {
	Month, Day, Hour, Minute, Second int
}

func (solarAnnual SolarAnnual) NextAfter(since time.Time) (time.Time, bool) {
	nextTime := makeSolarDate(solarAnnual, since.Year())
	if !nextTime.After(since) {
		nextTime = makeSolarDate(solarAnnual, since.Year()+1)
	}
	return nextTime, true
}

func makeSolarDate(solarAnnual SolarAnnual, year int) time.Time {
	nextTime := time.Date(year, time.Month(solarAnnual.Month), solarAnnual.Day, solarAnnual.Hour, solarAnnual.Minute, solarAnnual.Second, 0, time.Local)
	if nextTime.Month() != time.Month(solarAnnual.Month) {
		nextTime = nextTime.AddDate(0, 0, -1)
	}
	return nextTime
}
