// Package engine is used to schedule the notify nodes
package engine

import (
	"context"
	"log"
	"time"

	"butler/internal/model"
	"butler/internal/notify"
)

func Run(ctx context.Context, registry *notify.Registry, nodes []*model.Node) {
	internal := 1 * time.Minute
	for {
		now := time.Now()
		var soonest time.Time
		var target *model.Node
		// find the sonnest node to notify
		for _, node := range nodes {
			next, ok := node.Schedule.NextAfter(now)
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
		duration = min(duration, internal)
		timer := time.NewTimer(duration)
		select {
		case <-timer.C:
			// todo: 用发送标志而非时序判断发送
			// 这是个临时做法，正确做法其实是在node上打标，不过现在MVP就先这么做着
			if !time.Now().Before(soonest) {
				message := notify.Message{Title: target.Title, Body: target.Body}
				err := notify.Broadcast(ctx, registry, target.Channels, message)
				if err != nil {
					log.Printf("broadcast error: %w", err)
				}
			}
		case <-ctx.Done():
			timer.Stop()
			return
		}
	}
}
