package main

import (
	"log"

	"butler/internal/cli"

	"github.com/6tail/lunar-go/calendar"
)

func main() {
	cli.Execute()
}

func lunar() {
	lunar := calendar.NewLunarFromYmd(2026, 4, 24)
	solar := lunar.GetSolar()
	log.Println("any time sir ~")
	log.Println(solar.ToYmd())
}
