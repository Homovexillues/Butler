package notify

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

type Request struct {
	Channels []string
	Message  Message
	Result   chan error
}

func MessageLoop(ctx context.Context, registry *Registry, requests <-chan Request, workers int) {
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			for {
				select {
				case request := <-requests:
					err := broadcast(ctx, registry, request.Channels, request.Message)
					request.Result <- err
				case <-ctx.Done():
					return
				}
			}
		}()
	}
	wg.Wait()
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
	// 任意通知成功即为成功
	if len(joined) < len(channels) {
		return nil
	}
	return errors.Join(joined...)
}
