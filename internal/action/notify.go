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
		Result:   make(chan error, 1),
	}
	select {
	case requests <- request:
		select {
		case err := <-request.Result:
			return err
		}
	case <-ctx.Done():
		return ctx.Err()
	}
}
