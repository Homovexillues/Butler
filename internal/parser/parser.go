// Package parser is to Load config from jsonc files
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

func TaskToNode(task model.Task) (model.Node, error) {
	var node model.Node
	var taskSchedule schedule.Schedule
	var taskAction action.Action
	switch task.ScheduleType {
	case schedule.ScheduleOnce:
		at, err := time.Parse("2006-01-01 15:04:05", task.ScheduleExpression)
		if err != nil {
			return node, err
		}
		taskSchedule = schedule.Once{
			At: at,
		}
	case schedule.ScheduleSolar:
		at, err := time.Parse("2006-01-01 15:04:05", task.ScheduleExpression)
		if err != nil {
			return node, err
		}
		taskSchedule = schedule.SolarAnnual{
			Month:  int(at.Month()),
			Day:    at.Day(),
			Hour:   at.Hour(),
			Minute: at.Minute(),
			Second: at.Second(),
		}
	case schedule.ScheduleLunar:
		at, err := time.Parse("2006-01-01 15:04:05", task.ScheduleExpression)
		if err != nil {
			return node, err
		}
		taskSchedule = schedule.LunarSchedule{
			Month:  int(at.Month()),
			Day:    at.Day(),
			Hour:   at.Hour(),
			Minute: at.Minute(),
			Second: at.Second(),
		}
	case schedule.ScheduleCron:
		cronSchedule, err := schedule.NewCronSchedule(task.ScheduleExpression)
		if err != nil {
			return node, err
		}
		taskSchedule = cronSchedule
	case schedule.ScheduleInterval:
		intervalSchedule, err := schedule.NewIntervalSchedule(task.ScheduleExpression)
		if err != nil {
			return node, err
		}
		taskSchedule = intervalSchedule
	default:
		return node, fmt.Errorf("unknown schedule type %q", task.ScheduleType)
	}
	switch task.ActionType {
	case action.ActionNotify:
		notifyAction := action.NotifyAction{
			Channels: task.Channels,
			Message: notify.Message{
				Title: task.Title,
				Body:  task.Body,
			},
		}
		taskAction = notifyAction
	case action.ActionCommand:
		var commandAction action.CommandAction
		if err := json.Unmarshal(task.ActionExpression, commandAction); err != nil {
			return node, err
		}
		taskAction = commandAction
	default:
		return node, fmt.Errorf("unknown action type %q", task.ActionType)
	}
	node = model.Node{
		ID:        task.ID,
		Title:     task.Title,
		Body:      task.Body,
		Schedule:  taskSchedule,
		Action:    taskAction,
		LastFired: task.LastFired,
	}
	return node, nil
}

func PlanToNodes(plan *PlanNode) ([]*model.Node, error) {
	var out []*model.Node
	// 如果用index,item 的写法的话会创建副本
	for i := range plan.Children {
		err := walk(&plan.Children[i], &out)
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}

func walk(planNode *PlanNode, out *[]*model.Node) error {
	if len(planNode.Children) > 0 {
		for i := range planNode.Children {
			err := walk(&planNode.Children[i], out)
			if err != nil {
				return err
			}
		}
		return nil
	}

	node, err := planNode.toNode()
	if err != nil {
		return err
	}
	*out = append(*out, &node)
	return nil
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
