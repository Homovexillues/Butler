package action

import (
	"context"

	"butler/internal/notify"
)

type Action interface {
	Execute(ctx context.Context, message notify.Message) error
}
