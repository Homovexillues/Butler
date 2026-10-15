// Package action is
package action

import (
	"context"
)

type ActionType string

const (
	ActionCommand ActionType = "command"
)

func AllActionTypes() []ActionType {
	return []ActionType{
		ActionCommand,
	}
}

type Result struct {
	Output string
}

type Action interface {
	Execute(ctx context.Context) (Result, error)
}
