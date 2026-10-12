// Package engine is used to schedule the notify nodes
package engine

import (
	"context"
	"log/slog"
	"time"

	"butler/internal/model"
	"butler/internal/notify"
)

func Next(nodes []*model.Node, running map[int64]bool, now time.Time) (index int, at time.Time, found bool) {
	index = -1
	for i, node := range nodes {
		if node == nil || node.Schedule == nil || running[node.ID] {
			continue
		}
		// 	有执行记录的时候，从上次执行时间寻找下一次触发
		// 	没有的时候从调用方提供的时间寻找下一次触发
		since := now
		if node.LastFired != nil && !node.LastFired.IsZero() {
			since = *node.LastFired
		}

		next, ok := node.Schedule.NextAfter(since)
		if !ok {
			continue
		}

		if !found || next.Before(at) {
			index = i
			at = next
			found = true
		}
	}
	return index, at, found
}

func Run(ctx context.Context, nodesChannel <-chan []*model.Node, requests chan<- notify.Request, saveState func(id int64, lastFired time.Time) error) {
	nodes := []*model.Node{}
	results := make(chan actionResult, 10)
	running := make(map[int64]bool)
	for {
		var soonest time.Time
		now := time.Now()
		index, soonest, found := Next(nodes, running, now)
		var timer *time.Timer
		var timeChannel <-chan time.Time
		var target *model.Node
		if found {
			target = nodes[index]
			timer = time.NewTimer(max(time.Duration(0), soonest.Sub(now)))
			timeChannel = timer.C
		}

		select {
		case newNodes, ok := <-nodesChannel:
			if !ok {
				nodesChannel = nil
				break
			}
			// 在更新节点的时候保持已经开始执行过的任务的目标节点
			// 因为之前是直接加载配置文件来修改节点的，但是现在要热重载，所以要保留状态
			anchor := time.Now()
			nextNodes := make([]*model.Node, 0, len(nodes))
			for _, newNode := range newNodes {
				if newNode == nil {
					continue
				}
				node := *newNode
				fired := anchor
				if newNode.LastFired != nil && !newNode.LastFired.IsZero() {
					fired = *newNode.LastFired
				} else {
					for _, old := range nodes {
						if old.ID == node.ID && old.LastFired != nil {
							fired = *old.LastFired
							break
						}
					}
				}
				node.LastFired = &fired
				nextNodes = append(nextNodes, &node)
			}
			nodes = nextNodes
		case <-timeChannel:
			if !time.Now().Before(soonest) {
				running[target.ID] = true
				startAction(ctx, target, soonest, requests, results)
			}
		case result := <-results:
			if timer != nil {
				timer.Stop()
			}
			delete(running, result.Node.ID)
			if result.Err != nil {
				slog.Error("action failed", "action", result.Node.Title, "error", result.Err)
				continue
			} else {
				slog.Info("action succeed", "title", result.Node.Title)
			}

			fired := result.FinishedAt
			for _, node := range nodes {
				if node.ID == result.Node.ID {
					node.LastFired = &fired
					break
				}
			}

			if err := saveState(result.Node.ID, fired); err != nil {
				slog.ErrorContext(ctx, "fail to save state", "error", err)
			}
		case <-ctx.Done():
			if timer != nil {
				timer.Stop()
			}
			return
		}
	}
}
