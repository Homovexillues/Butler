package parser

import (
	"fmt"
	"time"

	"butler/internal/model"
	"butler/internal/schedule"
)

type Plan struct {
	Children []PlanNode
}

type PlanNode struct {
	Title        string
	Body         string
	Once         string
	Solar        string
	Lunar        string
	Cron         string
	NotifyOffset []string
	Channels     []string
	Children     []PlanNode
}

func (planNode PlanNode) toNode(channels []string) (model.Node, error) {
	if planNode.Title == "" && planNode.Body == "" {
		return model.Node{}, fmt.Errorf("title and body can not both be empty")
	}
	var result schedule.Schedule
	var err error
	switch {
	case planNode.Once != "":
		t, err := time.ParseInLocation("2006-01-02 15:04:05", planNode.Once, time.Local)
		if err != nil {
			return model.Node{}, err
		}
		once := schedule.Once{At: t}
		result = once
		//return once, nil
	case planNode.Lunar != "":
		t, err := time.Parse("01-02 15:04:05", planNode.Lunar)
		if err != nil {
			return model.Node{}, err
		}
		lunar := schedule.LunarSchedule{
			Month:  int(t.Month()),
			Day:    t.Day(),
			Hour:   t.Hour(),
			Minute: t.Minute(),
			Second: t.Second(),
		}
		result = lunar
		//return lunar, nil
	case planNode.Solar != "":
		t, err := time.Parse("01-02 15:04:05", planNode.Solar)
		if err != nil {
			return model.Node{}, err
		}
		solar := schedule.SolarAnnual{
			Month:  int(t.Month()),
			Day:    t.Day(),
			Hour:   t.Hour(),
			Minute: t.Minute(),
			Second: t.Second(),
		}
		result = solar
		//return solar, nil
	case planNode.Cron != "":
		cron, err := schedule.NewCronSchedule(planNode.Cron)
		if err != nil {
			return model.Node{}, err
		}
		result = cron
		//return cron, nil
	default:
		return model.Node{}, fmt.Errorf("no valid schedule configured")
	}
	if len(planNode.NotifyOffset) > 0 {
		result, err = toOffsetSchedule(result, planNode.NotifyOffset)
		if err != nil {
			return model.Node{}, err
		}
	}
	node := model.Node{
		Title:    planNode.Title,
		Body:     planNode.Body,
		Channels: channels,
		Schedule: result,
	}

	return node, nil
}

func KnowChannels() []string {
	return []string{"system", "mqtt", "email"}
}

func (plan Plan) ValidatePlan() []error {
	return []error{}
}
