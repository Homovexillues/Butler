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

func KnownChannels() []string {
	return []string{"system", "mqtt", "email", "messagebox"}
}

func (plan Plan) ValidatePlan(known map[string]bool) []error {
	errs := []error{}

	var walk func(planNode PlanNode, title string, channels []string)
	walk = func(planNode PlanNode, title string, channels []string) {
		if title == "" {
			errs = append(errs, fmt.Errorf("[%s] has a empty title,oh you do not know which node is?JUST WRITE TITLE", title))
			return
		}
		// 有子节点的话就递归遍历
		hasChild := len(planNode.Children) > 0

		var validExpressionCount int
		for _, expression := range []string{planNode.Once, planNode.Lunar, planNode.Solar, planNode.Cron} {
			if expression != "" {
				validExpressionCount += 1
			}
		}

		switch {
		case validExpressionCount == 0 && !hasChild:
			errs = append(errs, fmt.Errorf("you must set at least one schedule expression on [%s]", title))
		case validExpressionCount > 0 && hasChild:
			errs = append(errs, fmt.Errorf("[%s] node has child,can not use this node as root node", title))
		case validExpressionCount > 1:
			errs = append(errs, fmt.Errorf("you got %d schedule expression on [%s]", validExpressionCount, title))
		default:
			break
		}

		effective := planNode.Channels
		if len(effective) == 0 {
			effective = channels
		}
		for _, ch := range effective {
			if _, ok := known[ch]; !ok {
				errs = append(errs, fmt.Errorf("[%s] unknown channel %q", title, ch))
			}
		}
		if hasChild {
			for _, child := range planNode.Children {
				walk(child, planNode.Title+"/"+child.Title, effective)
			}
		} else {
			if len(effective) == 0 {
				errs = append(errs, fmt.Errorf("[%s] node and its parent has no channel configured", title))
			}
		}
		if validExpressionCount == 1 {
			_, err := planNode.toNode(effective)
			if err != nil {
				errs = append(errs, fmt.Errorf("[%s]'s schedule expression is invalid", title))
			}
		}
	}
	// 遍历所有子节点
	for _, child := range plan.Children {
		walk(child, child.Title, child.Channels)
	}
	return errs
}
