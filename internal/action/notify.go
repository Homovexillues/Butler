package action

import (
	"context"

	"butler/internal/notify"
)

type NotifyAction struct {
	Registry *notify.Registry
	Channels []string
}

func (a *NotifyAction) Execute(ctx context.Context, message notify.Message) error {
	return notify.Broadcast(ctx, a.Registry, a.Channels, message)
}
