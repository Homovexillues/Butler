package notify

import "context"

type EmailNotifier struct{}

func (email EmailNotifier) Name() string {
	return "email"
}

func (email EmailNotifier) Send(ctx context.Context, message Message) error {
	return nil
}
