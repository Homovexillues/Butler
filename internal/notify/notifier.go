// Package notify is a package to abstract the notification behavior
package notify

import "context"

type Message struct {
	Title string
	Body  string
}

type Notifier interface {
	Name() string
	Send(ctx context.Context, message Message) error
}
