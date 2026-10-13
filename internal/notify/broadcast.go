package notify

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"
)

type Request struct {
	Channels []Channel
	Message  Message
}
type notifyResult struct {
	Channel Channel
	Err     error
}

func Broadcast(ctx context.Context, registry *Registry, channels []Channel, message Message) error {
	var wg sync.WaitGroup
	results := make(chan notifyResult, len(channels))
	for _, name := range channels {
		wg.Add(1)
		go func(name Channel) {
			defer wg.Done()
			notifier, ok := registry.Get(name)
			if !ok {
				results <- notifyResult{name, fmt.Errorf("Channel %s not register", name)}
				return
			}
			err := notifier.Send(ctx, message)
			if err != nil {
				results <- notifyResult{Channel(notifier.Name()), fmt.Errorf("fail to send message throw %s channel:\n%w",
					notifier.Name(), err)}
			} else {
				results <- notifyResult{Channel(notifier.Name()), nil}
			}
		}(name)
	}
	wg.Wait()
	close(results)

	succeeded := make(map[Channel]bool)
	failed := make(map[Channel]bool)
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
			reportTo = append(reportTo, channel.String())
		}
	} else {
		for _, channel := range AllChannels() {
			if !slices.Contains(channels, Channel(channel)) {
				reportTo = append(reportTo, channel)
			}
		}
	}

	var reportError error
	for _, channel := range reportTo {
		if notifier, ok := registry.Get(Channel(channel)); ok {
			if err := notifier.Send(ctx, Message{
				Title: message.Title + "(含失败报告)",
				Body:  message.Body + fmt.Sprintf("\n%s", failedError.Error()),
			}); err != nil {
				reportError = errors.Join(reportError, err)
			} else {
				succeeded[Channel(channel)] = true
			}
		}
	}
	if len(succeeded) == 0 {
		return errors.Join(failedError, reportError)
	}
	return nil
}
