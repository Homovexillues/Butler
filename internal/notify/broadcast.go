package notify

import (
	"context"
	"log"
	"sync"
)

func Broadcast(ctx context.Context, registry *Registry, channels []string, message Message) {
	var wg sync.WaitGroup
	for _, name := range channels {
		notifier, ok := registry.Get(name)
		if !ok {
			log.Printf("Channel %s not register", name)
			continue
		}
		wg.Add(1)
		go func(notifier Notifier) {
			defer wg.Done()
			err := notifier.Send(ctx, message)
			if err != nil {
				log.Printf("Fail to send message throw %s channel:\n%s", notifier.Name(), err.Error())
			}
		}(notifier)
	}
	wg.Wait()
}
