// Package parser is to Load config from jsonc files
package parser

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
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
			if err != nil {
				return err
			}
		}
		return nil
	}

	node, err := planNode.toNode(channels)
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
	timeString := strings.Split(s, "T")[1]
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
	default:
		return 0, fmt.Errorf("not supported time char")
	}
}
