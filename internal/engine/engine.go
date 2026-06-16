// Package engine is used to schedule the notify nodes
package engine

import (
	"context"
	"time"

	"butler/internal/model"
	"butler/internal/notify"
)

func Run(ctx context.Context, nodes []*model.Node, notifier notify.Notifier) {
	maxTick := 1 * time.Minute
	for {
		now := time.Now()
		var soonest time.Time
		var target *model.Node
		// find the sonnest node to notify
		for _, node := range nodes {
			next, ok := node.Schedule.Next(now)
			if !ok {
				continue
			}
			if target == nil || next.Before(soonest) {
				soonest, target = next, node
			}
		}
		// no node to notify at all
		if target == nil {
			return
		}
		duration := time.Until(soonest)
		duration = min(duration, maxTick)
		timer := time.NewTimer(duration)
		select {
		case <-timer.C:
			_ = notifier.Send(notify.Message{Title: target.Title, Body: target.Title})
		case <-ctx.Done():
			timer.Stop()
			return
		}
	}
}
