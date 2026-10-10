package model

import (
	"encoding/json"
	"time"

	"butler/internal/action"
	"butler/internal/notify"
	"butler/internal/schedule"
)

type Task struct {
	ID                 int64                 `json:"id" gorm:"column:id;primaryKey;autoIncrement"`
	Title              string                `json:"title" gorm:"column:title;not null"`
	Body               string                `json:"body" gorm:"column:body;not null"`
	ScheduleType       schedule.ScheduleType `json:"scheduleType" gorm:"column:schedule_type;not null"`
	ScheduleOffset     []string              `json:"scheduleOffset" gorm:"column:schedule_offset;serializer:json"`
	ScheduleExpression string                `json:"scheduleExpression" gorm:"column:schedule_expression;not null"`
	ActionType         action.ActionType     `json:"actionType" gorm:"column:action_type;not null"`
	ActionExpression   json.RawMessage       `json:"actionExpression" gorm:"column:action_expression;not null"`
	Channel            notify.Channel        `json:"channel"`
	LastFired          *time.Time            `json:"lastFired" gorm:"column:last_fired"`
	Enabled            bool                  `json:"enabled" gorm:"column:enabled"`
}
