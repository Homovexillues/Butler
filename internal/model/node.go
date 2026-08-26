// Package model defines the core data type for butler
package model

import (
	"time"

	"butler/internal/action"
	"butler/internal/schedule"
)

type Node struct {
	Title     string
	Body      string
	Schedule  schedule.Schedule
	Action    action.Action
	LastFired time.Time
}
