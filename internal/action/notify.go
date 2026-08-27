package action

import (
	"context"

	"butler/internal/notify"
)

type NotifyAction struct {
	Channels []string       `json:"channels"`
	Message  notify.Message `json:"-"`
}

func (a NotifyAction) Execute(ctx context.Context, requests chan<- notify.Request) error {
	request := notify.Request{
		Channels: a.Channels,
		Message:  a.Message,
	}
	select {
	case requests <- request:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
