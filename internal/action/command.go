package action

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"

	"butler/internal/notify"
)

type CommandAction struct {
	Command string
	Args    []string
	Dir     string
	Env     map[string]string
}

func (a CommandAction) Execute(ctx context.Context, requests chan<- notify.Request) error {
	request := notify.Request{
		Channels: []string{notify.ChannelMQTT.String()},
		Result:   make(chan error, 1),
	}

	if a.Command == "" {
		return fmt.Errorf("command program is empty")
	}

	cmd := exec.CommandContext(ctx, a.Command, a.Args...)

	if a.Dir != "" {
		cmd.Dir = a.Dir
	}

	cmd.Env = os.Environ()
	for key, value := range a.Env {
		cmd.Env = append(cmd.Env, key+"="+value)
	}
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf(
			"execute %q: %w\noutput:\n%s",
			a.Command,
			err,
			output)
	}
	if len(output) > 0 {
		slog.Info("command executed", "command", a.Command, "output", string(output))
		request.Message.Title = fmt.Sprintf("command %s execute result", a.Command)
		request.Message.Body = string(output)
		select {
		case requests <- request:
			select {
			case err := <-request.Result:
				if err != nil {
					slog.Error("command succeeded but notification failed", "command", a.Command, "error", err)
				}
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}
