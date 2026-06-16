package notify

import (
	"fmt"

	"github.com/gen2brain/beeep"
)

type SystemNotifier struct{}

func (sys SystemNotifier) Name() string {
	return "SystemNotifier"
}

func (sys SystemNotifier) Send(message Message) error {
	err := beeep.Notify("Notify", message.Body, "")
	if err != nil {
		return fmt.Errorf("system notify error: %w", err)
	}
	return nil
}
