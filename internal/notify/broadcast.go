package notify

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"
)

type Request struct {
	Channels []string
	Message  Message
	Result   chan error
}
type notifyResult struct {
	Channel string
	Err     error
}

func MessageLoop(ctx context.Context, registry *Registry, requests <-chan Request, workers int) {
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
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
	results := make(chan notifyResult, len(channels))
	for _, name := range channels {
		wg.Add(1)
		go func(name string) {
			defer wg.Done()
			notifier, ok := registry.Get(name)
			if !ok {
				results <- notifyResult{name, fmt.Errorf("Channel %s not register", name)}
				return
			}
			err := notifier.Send(ctx, message)
			if err != nil {
				results <- notifyResult{notifier.Name(), fmt.Errorf("Fail to send message throw %s channel:\n%w",
					notifier.Name(), err)}
			} else {
				results <- notifyResult{notifier.Name(), nil}
			}
		}(name)
	}
	wg.Wait()
	close(results)

	succeeded := make(map[string]bool)
	failed := make(map[string]bool)
	var failedError error
	for result := range results {
		if result.Err != nil {
			failed[result.Channel] = false
			failedError = errors.Join(failedError, result.Err)
			continue
		}
		succeeded[result.Channel] = true
	}
	if len(failed) == 0 {
		return nil
	}
	reportTo := make([]string, 0, len(channels))
	if len(succeeded) > 0 {
		for channel := range succeeded {
			reportTo = append(reportTo, channel)
		}
	} else {
		for _, channel := range AllChannels() {
			if !slices.Contains(channels, channel) {
				reportTo = append(reportTo, channel)
			}
		}
	}

	var reportError error
	for _, channel := range reportTo {
		if notifier, ok := registry.Get(channel); ok {
			if err := notifier.Send(ctx, Message{
				Title: message.Title + "(含失败报告)",
				Body:  message.Body + fmt.Sprintf("\n%s", failedError.Error()),
			}); err != nil {
				reportError = errors.Join(reportError, err)
			}
		}
	}
	if len(succeeded) == 0 {
		return errors.Join(failedError, reportError)
	}
	return nil
}
