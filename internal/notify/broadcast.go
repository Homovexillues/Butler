package notify

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

func Broadcast(ctx context.Context, registry *Registry, channels []string, message Message) error {
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
