package action

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Run the test binary as a helper, without depending on a platform shell.
func TestCommandActionHelperProcess(t *testing.T) {
	if os.Getenv("BUTLER_COMMAND_HELPER") != "1" {
		return
	}
	if _, err := fmt.Fprint(os.Stdout, os.Getenv("BUTLER_COMMAND_OUTPUT")); err != nil {
		t.Fatalf("write helper stdout: %v", err)
	}
	if _, err := fmt.Fprint(os.Stderr, os.Getenv("BUTLER_COMMAND_STDERR")); err != nil {
		t.Fatalf("write helper stderr: %v", err)
	}
	if os.Getenv("BUTLER_COMMAND_CWD") == "1" {
		cwd, err := os.Getwd()
		if err != nil {
			t.Fatalf("get helper working directory: %v", err)
		}
		if _, err := fmt.Fprint(os.Stdout, cwd); err != nil {
			t.Fatalf("write helper working directory: %v", err)
		}
	}
	if os.Getenv("BUTLER_COMMAND_FAIL") == "1" {
		os.Exit(2)
	}
	os.Exit(0)
}

func helperCommandAction(output string, fail bool) CommandAction {
	env := map[string]string{"BUTLER_COMMAND_HELPER": "1", "BUTLER_COMMAND_OUTPUT": output}
	if fail {
		env["BUTLER_COMMAND_FAIL"] = "1"
	}
	return CommandAction{Command: os.Args[0], Args: []string{"-test.run=^TestCommandActionHelperProcess$"}, Env: env}
}

func TestCommandActionReturnsOutput(t *testing.T) {
	for _, output := range []string{"", "采集完成"} {
		t.Run(output, func(t *testing.T) {
			result, err := helperCommandAction(output, false).Execute(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			if result.Output != output {
				t.Fatalf("output = %q, want %q", result.Output, output)
			}
		})
	}
}

func TestCommandActionFailurePreservesOutputAndExitError(t *testing.T) {
	command := helperCommandAction("partial output", true)
	command.Env["BUTLER_COMMAND_STDERR"] = "failure detail"
	result, err := command.Execute(context.Background())
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) || exitErr.ExitCode() != 2 {
		t.Fatalf("expected wrapped exit code 2: %v", err)
	}
	for _, part := range []string{"partial output", "failure detail"} {
		if !strings.Contains(result.Output, part) {
			t.Fatalf("missing %q in output %q", part, result.Output)
		}
	}
}

func TestCommandActionWorkingDirectory(t *testing.T) {
	dir := t.TempDir()
	command := helperCommandAction("", false)
	command.Dir = dir
	command.Env["BUTLER_COMMAND_CWD"] = "1"
	result, err := command.Execute(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	want, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatal(err)
	}
	if result.Output != want {
		t.Fatalf("cwd = %q, want %q", result.Output, want)
	}
}

func TestCommandActionRejectsEmptyCommand(t *testing.T) {
	if _, err := (CommandAction{}).Execute(context.Background()); err == nil {
		t.Fatal("empty command accepted")
	}
}

func TestCommandActionCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := helperCommandAction("should not run", false).Execute(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected cancellation, got %v", err)
	}
}
