// Package notify is a package to abstract the notification behavior
package notify

type Message struct {
	Title string
	Body  string
}

type Notifier interface {
	Name() string
	Send(message Message) error
}
