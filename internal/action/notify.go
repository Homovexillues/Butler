package action

import (
	"context"

	"butler/internal/notify"
)

type NotifyAction struct {
	Channels []notify.Channel `json:"channels"`
	Message  notify.Message   `json:"-"`
}

func (a NotifyAction) Execute(ctx context.Context, requests chan<- notify.Request) error {
	channels := make([]string, len(a.Channels))
	for i, channel := range a.Channels {
		channels[i] = channel.String()
	}
	request := notify.Request{
		Channels: channels,
		Message:  a.Message,
		Result:   make(chan error, 1),
	}
	select {
	case requests <- request:
		return <-request.Result
	case <-ctx.Done():
		return ctx.Err()
	}
}
