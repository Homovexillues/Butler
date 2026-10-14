package notify

import "fmt"

type Registry struct {
	channels map[Channel]Notifier
}

func NewRegistry() *Registry {
	return &Registry{channels: make(map[Channel]Notifier)}
}

func (registry *Registry) Register(notifier Notifier) error {
	if notifier == nil {
		return fmt.Errorf("notifier is nil")
	}
	ch, ok := ParseChannel(notifier.Name())
	if !ok {
		return fmt.Errorf("invalid notifier name %q", notifier.Name())
	}
	if _, exists := registry.channels[ch]; exists {
		return fmt.Errorf("channel %q already registered", ch)
	}
	registry.channels[ch] = notifier
	return nil
}

func (registry *Registry) Get(name Channel) (Notifier, bool) {
	notifier, ok := registry.channels[name]
	return notifier, ok
}
