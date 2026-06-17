package notify

type Registry struct {
	channels map[string]Notifier
}

func NewRegistry() *Registry {
	return &Registry{channels: make(map[string]Notifier)}
}

func (registry *Registry) Register(notifier Notifier) {
	registry.channels[notifier.Name()] = notifier
}

func (registry *Registry) Get(name string) (Notifier, bool) {
	notifier, ok := registry.channels[name]
	return notifier, ok
}
