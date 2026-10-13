// Package model defines the core data type for butler
package model

import (
	"time"

	"butler/internal/action"
	"butler/internal/notify"
	"butler/internal/schedule"
)

type Node struct {
	ID        int64
	Title     string
	Body      string
	Schedule  schedule.Schedule
	Channels  []notify.Channel
	Action    action.Action
	LastFired *time.Time
}
