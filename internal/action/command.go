package action

import (
	"context"
	"fmt"
	"os"
	"os/exec"
)

type CommandAction struct {
	Command string
	Args    []string
	Dir     string
	Env     map[string]string
}

func (a CommandAction) Execute(ctx context.Context) (Result, error) {
	if a.Command == "" {
		errorInfo := "command program is empty"
		return Result{Output: errorInfo}, fmt.Errorf(errorInfo)
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
		return Result{
			Output: string(output),
		}, fmt.Errorf("fail to execute %q:\n %w", a.Command, err)
	}
	return Result{Output: string(output)}, nil
}
