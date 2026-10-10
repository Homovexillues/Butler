// Package action is
package action

import (
	"context"

	"butler/internal/notify"
)

type ActionType string

const (
	ActionNotify  ActionType = "notify"
	ActionCommand ActionType = "command"
)

type Action interface {
	Execute(ctx context.Context, messageChannel chan<- notify.Request) error
}
