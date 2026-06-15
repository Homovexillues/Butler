package main

import (
	"log"

	"butler/internal/notify"

	"github.com/6tail/lunar-go/calendar"
)

func main() {
	notifier := notify.SystemNotifier{}
	message := notify.Message{
		Title: "Notify",
		Body:  "记得下班打卡",
	}
	notifier.Send(message)
}

func lunar() {
	lunar := calendar.NewLunarFromYmd(2026, 4, 24)
	solar := lunar.GetSolar()
	log.Println("any time sir ~")
	log.Println(solar.ToYmd())
}
