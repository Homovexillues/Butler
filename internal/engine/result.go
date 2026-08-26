package engine

import (
	"context"
	"time"

	"butler/internal/model"
	"butler/internal/notify"
)

type actionResult struct {
	Node        *model.Node
	ScheduledAt time.Time
	StartedAt   time.Time
	FinishedAt  time.Time
	Err         error
}

func startAction(ctx context.Context, node *model.Node, scheduledAt time.Time, requests chan<- notify.Request, results chan<- actionResult) {
	go func() {
		startedAt := time.Now()

		err := node.Action.Execute(ctx, requests)
		result := actionResult{
			Node:        node,
			ScheduledAt: scheduledAt,
			StartedAt:   startedAt,
			FinishedAt:  time.Now(),
			Err:         err,
		}
		select {
		case results <- result:
		case <-ctx.Done():
		}
	}()
}
