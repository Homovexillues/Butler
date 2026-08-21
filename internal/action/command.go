package action

import (
	"context"
	"fmt"
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

func (a *CommandAction) Execute(ctx context.Context, message notify.Message) error {
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
			output,
		)
	}

	return nil
}
