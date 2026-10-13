package notify

type Registry struct {
	channels map[Channel]Notifier
}

func NewRegistry() *Registry {
	return &Registry{channels: make(map[Channel]Notifier)}
}

func (registry *Registry) Register(notifier Notifier) {
	registry.channels[Channel(notifier.Name())] = notifier
}

func (registry *Registry) Get(name Channel) (Notifier, bool) {
	notifier, ok := registry.channels[name]
	return notifier, ok
}
