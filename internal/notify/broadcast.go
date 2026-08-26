package notify

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"
)

type Request struct {
	Channels []string
	Message  Message
}

func MessageLoop(ctx context.Context, registry *Registry, requests <-chan Request) {
	for {
		select {
		case request := <-requests:
			var err error
			err = broadcast(ctx, registry, request.Channels, request.Message)
			if err != nil {
				log.Printf("fail to broadcast message: %v", err)
			}
		case <-ctx.Done():
			return
		}
	}
}

func broadcast(ctx context.Context, registry *Registry, channels []string, message Message) error {
	var wg sync.WaitGroup
	errs := make(chan error, len(channels))
	for _, name := range channels {
		notifier, ok := registry.Get(name)
		if !ok {
			errs <- fmt.Errorf("Channel %s not register", name)
			continue
		}
		wg.Add(1)
		go func(notifier Notifier) {
			defer wg.Done()
			err := notifier.Send(ctx, message)
			if err != nil {
				errs <- fmt.Errorf("Fail to send message throw %s channel:\n%w",
					notifier.Name(), err)
			}
		}(notifier)
	}
	wg.Wait()
	close(errs)
	var joined []error
	for err := range errs {
		joined = append(joined, err)
	}
	return errors.Join(joined...)
}
