package notify

import (
	"context"
	"fmt"

	"github.com/gen2brain/beeep"
)

type SystemNotifier struct{}

func (sys SystemNotifier) Name() string {
	return "system"
}

func (sys SystemNotifier) Send(ctx context.Context, message Message) error {
	err := beeep.Beep(400, 500)
	if err != nil {
		return fmt.Errorf("system notify error: %w", err)
	}
	err = beeep.Notify("Notify", message.Body, "")
	if err != nil {
		return fmt.Errorf("system notify error: %w", err)
	}
	return nil
}
