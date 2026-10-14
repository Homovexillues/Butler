// Package parser is to load config from jsonc files
package parser

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"butler/internal/action"
	"butler/internal/model"
	"butler/internal/notify"
	"butler/internal/schedule"

	"github.com/tailscale/hujson"
)

func Parse[T any](path string) (T, error) {
	var result T
	raw, err := os.ReadFile(path)
	if err != nil {
		return result, err
	}
	if len(bytes.TrimSpace(raw)) == 0 {
		return result, fmt.Errorf("parse %s file is empty", path)
	}
	stdJSONData, err := hujson.Standardize(raw)
	if err != nil {
		return result, err
	}
	return result, json.Unmarshal(stdJSONData, &result)
}

func MakeSchedule(scheduleType schedule.ScheduleType, expression string, offset []string) (schedule.Schedule, error) {
	var taskSchedule schedule.Schedule
	switch scheduleType {
	case schedule.ScheduleOnce:
		at, err := time.Parse("2006-01-02 15:04:05", expression)
		if err != nil {
			return taskSchedule, err
		}
		taskSchedule = schedule.Once{
			At: at,
		}
	case schedule.ScheduleSolar:
		at, err := time.Parse("01-02 15:04:05", expression)
		if err != nil {
			return taskSchedule, err
		}
		taskSchedule = schedule.SolarAnnual{
			Month:  int(at.Month()),
			Day:    at.Day(),
			Hour:   at.Hour(),
			Minute: at.Minute(),
			Second: at.Second(),
		}
	case schedule.ScheduleLunar:
		at, err := time.Parse("01-02 15:04:05", expression)
		if err != nil {
			return taskSchedule, err
		}
		taskSchedule = schedule.LunarSchedule{
			Month:  int(at.Month()),
			Day:    at.Day(),
			Hour:   at.Hour(),
			Minute: at.Minute(),
			Second: at.Second(),
		}
	case schedule.ScheduleCron:
		cronSchedule, err := schedule.NewCronSchedule(expression)
		if err != nil {
			return taskSchedule, err
		}
		taskSchedule = cronSchedule
	case schedule.ScheduleInterval:
		intervalSchedule, err := schedule.NewIntervalSchedule(expression)
		if err != nil {
			return taskSchedule, err
		}
		taskSchedule = intervalSchedule
	default:
		return taskSchedule, fmt.Errorf("unknown schedule type %q", scheduleType)
	}
	if len(offset) != 0 {
		offsetSchedule, err := toOffsetSchedule(taskSchedule, offset)
		if err != nil {
			return nil, fmt.Errorf("invalid schedule offset: %w", err)
		}
		taskSchedule = offsetSchedule
	}
	return taskSchedule, nil
}

func MakeAction(actionType action.ActionType, expression string) (action.Action, error) {
	var taskAction action.Action
	switch actionType {
	case "":
		return nil, nil
	case action.ActionCommand:
		var commandAction action.CommandAction
		if err := json.Unmarshal([]byte(expression), &commandAction); err != nil {
			return taskAction, err
		}
		taskAction = commandAction
	default:
		return taskAction, fmt.Errorf("unknown action type %q", actionType)
	}
	return taskAction, nil
}

func TaskToNode(task model.Task) (model.Node, error) {
	var node model.Node
	if err := notify.ValidateChannels(task.Channels); err != nil {
		return node, fmt.Errorf("task %d: %w", task.ID, err)
	}
	taskSchedule, err := MakeSchedule(task.ScheduleType, task.ScheduleExpression, task.ScheduleOffset)
	if err != nil {
		return model.Node{}, err
	}
	taskAction, err := MakeAction(task.ActionType, task.ActionExpression.String())
	if err != nil {
		return model.Node{}, err
	}
	node = model.Node{
		ID:        task.ID,
		Title:     task.Title,
		Body:      task.Body,
		Schedule:  taskSchedule,
		Action:    taskAction,
		Channels:  task.Channels,
		LastFired: task.LastFired,
	}
	return node, nil
}

func toOffsetSchedule(s schedule.Schedule, notifyOffsets []string) (schedule.Schedule, error) {
	var offsets []time.Duration
	for _, notifyOffset := range notifyOffsets {
		offset, err := parseOffset(notifyOffset)
		if err != nil {
			return nil, err
		}
		offsets = append(offsets, offset)
	}
	offsetSchedule, err := schedule.NewOffsetSchedule(s, offsets)
	if err != nil {
		return nil, err
	}
	return offsetSchedule, nil
}

func parseOffset(s string) (time.Duration, error) {
	s = strings.ReplaceAll(s, " ", "")
	if s == "" || !strings.Contains(s, "T") {
		return 0, nil
	}
	timeString := strings.Split(s, "T")[1]
	if timeString == "" {
		return 0, nil
	}
	switch {
	case strings.HasSuffix(timeString, "d"):
		timeNumString, _, _ := strings.Cut(timeString, "d")
		num, err := strconv.Atoi(timeNumString)
		if err != nil {
			return 0, err
		}
		return time.Duration(num) * 24 * time.Hour, nil

	case strings.HasSuffix(timeString, "h"):
		timeNumString, _, _ := strings.Cut(timeString, "h")
		num, err := strconv.Atoi(timeNumString)
		if err != nil {
			return 0, err
		}
		return time.Duration(num) * time.Hour, nil
	case strings.HasSuffix(timeString, "m"):
		timeNumString, _, _ := strings.Cut(timeString, "m")
		num, err := strconv.Atoi(timeNumString)
		if err != nil {
			return 0, err
		}
		return time.Duration(num) * time.Minute, nil
	default:
		return 0, fmt.Errorf("not supported time char")
	}
}
