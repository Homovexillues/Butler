// Package parser is to Load config from jsonc files
package parser

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"butler/internal/model"
	"butler/internal/schedule"

	"github.com/tailscale/hujson"
)

func Parse[T any](path string) (T, error) {
	var result T
	raw, err := os.ReadFile(path)
	if err != nil {
		return result, err
	}
	stdJSONData, err := hujson.Standardize(raw)
	if err != nil {
		return result, err
	}
	return result, json.Unmarshal(stdJSONData, &result)
}

func planToSchedule(planNode PlanNode) (schedule.Schedule, error) {
	if planNode.Title == "" && planNode.Body == "" {
		return nil, fmt.Errorf("title and body can not both be empty")
	}
	switch {
	case planNode.Once != "":
		t, err := time.ParseInLocation("2006-01-02 15:04:05", planNode.Once, time.Local)
		if err != nil {
			return nil, err
		}
		once := schedule.Once{At: t}
		return once, nil
	case planNode.Lunar != "":
		fallthrough
	case planNode.Solar != "":
		t, err := time.Parse("2006-01-02 15:04:05", planNode.Solar)
		if err != nil {
			return nil, err
		}
		solar := schedule.SolarAnnual{
			Month:  int(t.Month()),
			Day:    t.Day(),
			Hour:   t.Hour(),
			Minute: t.Minute(),
			Second: t.Second(),
		}
		return solar, nil
	case planNode.Cron != "":
		cron, err := schedule.NewCronSchedule(planNode.Cron)
		if err != nil {
			return nil, err
		}
		return cron, nil
	default:
		return nil, fmt.Errorf("no valid schedule configured")
	}
}

func PlanToNodes(plan Plan) ([]*model.Node, error) {
	var out []*model.Node
	for _, child := range plan.Children {
		err := walk(child, nil, &out)
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}

func walk(planNode PlanNode, inherited []string, out *[]*model.Node) error {
	channels := planNode.Channels
	if len(channels) == 0 {
		channels = inherited
	}
	if len(planNode.Children) > 0 {
		for _, pNode := range planNode.Children {
			err := walk(pNode, channels, out)
			return err
		}
	}

	schedule, err := planToSchedule(planNode)
	if err != nil {
		return err
	}
	node := model.Node{
		Title:    planNode.Title,
		Body:     planNode.Body,
		Channels: planNode.Channels,
		Schedule: schedule,
	}
	*out = append(*out, &node)
	return nil
}
