package action

import (
	"context"

	"butler/internal/notify"
)

type Action interface {
	Execute(ctx context.Context, messageChannel chan<- notify.Request) error
}
