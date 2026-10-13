package engine

import (
	"context"
	"time"

	"butler/internal/action"
	"butler/internal/model"
)

type actionResult struct {
	Node        *model.Node
	Result      action.Result
	ScheduledAt time.Time
	StartedAt   time.Time
	FinishedAt  time.Time
	Err         error
}

func startAction(ctx context.Context, node *model.Node, scheduledAt time.Time, results chan<- actionResult) {
	go func() {
		var result action.Result
		var err error
		startedAt := time.Now()
		if node.Action != nil {
			result, err = node.Action.Execute(ctx)
		}
		select {
		case results <- actionResult{
			Node:        node,
			ScheduledAt: scheduledAt,
			Result:      result,
			StartedAt:   startedAt,
			FinishedAt:  time.Now(),
			Err:         err,
		}:
		case <-ctx.Done():
		}
	}()
}
