package notify

import (
	"context"

	"github.com/gen2brain/dlgs"
)

type MessageboxNotifier struct{}

func (messagebox MessageboxNotifier) Name() string {
	return "messagebox"
}

func (messagebox MessageboxNotifier) Send(ctx context.Context, message Message) error {
	_, err := dlgs.Info(message.Title, message.Body)
	return err
}
