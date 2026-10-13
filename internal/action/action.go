// Package action is
package action

import (
	"context"
)

type ActionType string

const (
	ActionCommand ActionType = "command"
)

type Result struct {
	Output string
}

type Action interface {
	Execute(ctx context.Context) (Result, error)
}
