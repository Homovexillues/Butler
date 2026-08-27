// Package engine is used to schedule the notify nodes
package engine

import (
	"context"
	"log"
	"time"

	"butler/internal/model"
	"butler/internal/notify"
)

func Run(ctx context.Context, nodes []*model.Node, requests chan<- notify.Request, saveState func() error) {
	internal := 1 * time.Minute
	results := make(chan actionResult, 10)
	running := make(map[*model.Node]bool)
	for {
		var soonest time.Time
		var target *model.Node
		// find the sonnest node to notify
		for _, node := range nodes {
			if running[node] {
				continue
			}
			// 如果有未发的任务，优先补发
			next, ok := node.Schedule.NextAfter(*node.LastFired)
			if !ok {
				continue
			}
			if target == nil || next.Before(soonest) {
				soonest, target = next, node
			}
		}
		var timer *time.Timer
		var timeChannel <-chan time.Time
		switch {
		case target != nil:
			duration := time.Until(soonest)
			duration = min(duration, internal)
			timer = time.NewTimer(duration)
			timeChannel = timer.C
		case len(running) == 0:
			return
		}

		select {
		case <-timeChannel:
			if !time.Now().Before(soonest) {
				running[target] = true
				startAction(ctx, target, soonest, requests, results)
			}
		case result := <-results:
			if timer != nil {
				timer.Stop()
			}
			delete(running, result.Node)
			if result.Err != nil {
				log.Printf(
					"action %q failed: %v",
					result.Node.Title,
					result.Err,
				)
				continue
			}
			*result.Node.LastFired = time.Now()
			if err := saveState(); err != nil {
				log.Printf("fail to save state: %v", err)
			}
		case <-ctx.Done():
			if timer != nil {
				timer.Stop()
			}
			return
		}
	}
}
