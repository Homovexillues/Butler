// Package model defines the core data type for butler
package model

import (
	"butler/internal/schedule"
)

type Node struct {
	Title    string
	Schedule schedule.Schedule
}
