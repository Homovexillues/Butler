package action

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"time"
)

type CommandAction struct {
	Command string
	Args    []string
	Dir     string
	Env     map[string]string
	Timeout time.Duration
}

func (a CommandAction) Execute(ctx context.Context) (Result, error) {
	// Timeout 为 0 表示不限制：WithTimeout(ctx, 0) 会得到一个已经过期的 ctx，
	// 命令会立刻失败。
	if a.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, a.Timeout)
		defer cancel()
	}
	if a.Command == "" {
		errorInfo := "command program is empty"
		return Result{Output: errorInfo}, errors.New(errorInfo)
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
