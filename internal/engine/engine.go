// Package engine is used to schedule the notify nodes
package engine

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"butler/internal/model"
	"butler/internal/notify"
)

func Since(lastAttempt, lastFired, now time.Time) time.Time {
	if then := later(lastAttempt, lastFired); !then.IsZero() {
		return then
	}
	return now
}

func later(a, b time.Time) time.Time {
	if a.Before(b) {
		return b
	}
	return a
}

func mergeNodes(old map[int64]*model.Node, newNodes []*model.Node, anchor time.Time) map[int64]*model.Node {
	next := make(map[int64]*model.Node, len(newNodes))
	for _, newNode := range newNodes {
		if newNode == nil {
			continue
		}
		node := *newNode
		if oldNode, ok := old[node.ID]; ok {
			node.LastAttempt = later(node.LastAttempt, oldNode.LastAttempt)
			node.LastFired = later(node.LastFired, oldNode.LastFired)
		}
		node.LastAttempt = later(node.LastAttempt, node.LastFired)
		if node.LastAttempt.IsZero() {
			node.LastAttempt = anchor
		}
		next[node.ID] = &node
	}
	return next
}

func Next(nodes map[int64]*model.Node, running map[int64]bool, now time.Time) (result *model.Node, at time.Time, found bool) {
	for id, node := range nodes {
		if node == nil || node.Schedule == nil || running[id] {
			continue
		}
		// 	有执行记录的时候，从上次执行时间寻找下一次触发
		// 	没有的时候从调用方提供的时间寻找下一次触发
		since := Since(node.LastAttempt, node.LastFired, now)
		next, ok := node.Schedule.NextAfter(since)
		if !ok {
			continue
		}

		if !found || next.Before(at) {
			result = node
			at = next
			found = true
		}
	}
	return result, at, found
}

func Run(ctx context.Context, nodesChannel <-chan []*model.Node, requests chan<- notify.Request, saveState func(id int64, lastAttempt *time.Time, lastFired *time.Time) error) {
	nodes := map[int64]*model.Node{}
	results := make(chan actionResult, 10)
	running := make(map[int64]bool)
	for {
		now := time.Now()
		nodeResult, soonest, found := Next(nodes, running, now)
		var timer *time.Timer
		var timeChannel <-chan time.Time
		var target *model.Node
		if found {
			target = nodeResult
			timer = time.NewTimer(max(time.Duration(0), soonest.Sub(now)))
			timeChannel = timer.C
		}

		select {
		case newNodes, ok := <-nodesChannel:
			if !ok {
				nodesChannel = nil
				break
			}
			// 更新配置时合并执行状态：数据库读到的是旧值也不能让内存状态回退。
			nodes = mergeNodes(nodes, newNodes, time.Now())
		case <-timeChannel:
			if !time.Now().Before(soonest) {
				running[target.ID] = true
				startAction(ctx, target, soonest, results)
			}
		case result := <-results:
			if timer != nil {
				timer.Stop()
			}
			attempt := result.FinishedAt
			var fired *time.Time
			if result.Err == nil {
				fired = &attempt
			}
			request := notify.Request{
				Channels: result.Node.Channels,
				Message: notify.Message{
					Title: result.Node.Title,
					Body:  result.Node.Body,
				},
			}
			delete(running, result.Node.ID)
			if result.Err != nil {
				slog.Error("action failed", "action", result.Node.Title, "error", result.Err)
				request.Message.Body += fmt.Sprintf("\n=action failed=\n%s %v", result.Result.Output, result.Err)
			} else if result.Node.Action != nil {
				slog.Info("action succeed", "title", result.Node.Title)
				request.Message.Body += fmt.Sprintf("\n=action succeed=\n %s", result.Result.Output)
			}
			select {
			case requests <- request:
			case <-ctx.Done():
				return
			}
			// The snapshot may have been replaced while the action was running;
			// update the node that is current now, not the pointer we started with.
			if node, ok := nodes[result.Node.ID]; ok {
				node.LastAttempt = attempt
				if fired != nil {
					node.LastFired = attempt
				}
			}
			if err := saveState(result.Node.ID, &attempt, fired); err != nil {
				slog.ErrorContext(ctx, "fail to save state", "error", err)
			}
		case <-ctx.Done():
			if timer != nil {
				timer.Stop()
			}
			return
		}

		if timer != nil {
			timer.Stop()
		}
	}
}
