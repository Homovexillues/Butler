package action

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"butler/internal/notify"
)

// TestCommandActionHelperProcess 由下面的测试把当前测试二进制作为子进程启动。
// 这样不依赖 bash、cmd.exe 等外部 shell，Linux 和 Windows 都能运行。
func TestCommandActionHelperProcess(t *testing.T) {
	if os.Getenv("BUTLER_COMMAND_HELPER") != "1" {
		return
	}

	fmt.Fprint(os.Stdout, os.Getenv("BUTLER_COMMAND_OUTPUT"))
	if os.Getenv("BUTLER_COMMAND_FAIL") == "1" {
		os.Exit(2)
	}
	os.Exit(0)
}

func TestCommandAction_命令成功且通知成功(t *testing.T) {
	action := helperCommandAction("采集完成", false)
	requests := make(chan notify.Request, 1)
	done := make(chan error, 1)

	go func() {
		done <- action.Execute(context.Background(), requests)
	}()

	request := waitCommandRequest(t, requests)
	if request.Message.Title != "command "+os.Args[0]+" execute result" {
		t.Errorf("通知标题 = %q", request.Message.Title)
	}
	if request.Message.Body != "采集完成" {
		t.Errorf("通知正文 = %q，期望 %q", request.Message.Body, "采集完成")
	}
	request.Result <- nil

	if err := waitCommandResult(t, done); err != nil {
		t.Fatalf("Execute() 返回错误：%v", err)
	}
}

func TestCommandAction_命令成功但通知失败仍返回成功(t *testing.T) {
	action := helperCommandAction("采集完成", false)
	requests := make(chan notify.Request, 1)
	done := make(chan error, 1)

	go func() {
		done <- action.Execute(context.Background(), requests)
	}()

	request := waitCommandRequest(t, requests)
	request.Result <- errors.New("所有通知渠道均不可用")

	if err := waitCommandResult(t, done); err != nil {
		t.Fatalf("命令已经成功，通知失败不应使 CommandAction 失败：%v", err)
	}
}

func TestCommandAction_命令失败返回错误且不发送通知(t *testing.T) {
	action := helperCommandAction("执行失败", true)
	requests := make(chan notify.Request, 1)

	err := action.Execute(context.Background(), requests)
	if err == nil {
		t.Fatal("命令退出码非零，Execute() 应返回错误")
	}
	if !strings.Contains(err.Error(), "执行失败") {
		t.Fatalf("错误没有保留命令输出：%v", err)
	}

	select {
	case request := <-requests:
		t.Fatalf("命令失败时不应发送成功结果通知，实际得到 %+v", request)
	default:
	}
}

func TestCommandAction_空命令返回错误(t *testing.T) {
	err := (CommandAction{}).Execute(context.Background(), make(chan notify.Request, 1))
	if err == nil {
		t.Fatal("空命令应返回错误")
	}
}

func helperCommandAction(output string, fail bool) CommandAction {
	env := map[string]string{
		"BUTLER_COMMAND_HELPER": "1",
		"BUTLER_COMMAND_OUTPUT": output,
	}
	if fail {
		env["BUTLER_COMMAND_FAIL"] = "1"
	}

	return CommandAction{
		Command: os.Args[0],
		Args: []string{
			"-test.run=^TestCommandActionHelperProcess$",
		},
		Env: env,
	}
}

func waitCommandRequest(t *testing.T, requests <-chan notify.Request) notify.Request {
	t.Helper()
	select {
	case request := <-requests:
		return request
	case <-time.After(2 * time.Second):
		t.Fatal("没有在超时前收到命令结果通知")
		return notify.Request{}
	}
}

func waitCommandResult(t *testing.T, done <-chan error) error {
	t.Helper()
	select {
	case err := <-done:
		return err
	case <-time.After(2 * time.Second):
		t.Fatal("CommandAction 没有在超时前结束")
		return nil
	}
}
